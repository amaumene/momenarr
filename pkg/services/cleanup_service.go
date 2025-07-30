package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/utils"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

// CleanupService handles cleanup of watched media with AllDebrid support
type CleanupService struct {
	repo             repository.Repository
	allDebridService AllDebridInterface
	token            *trakt.Token
	watchedDays      int
}

// CreateCleanupService creates a cleanup service
func CreateCleanupService(repo repository.Repository, allDebrid AllDebridInterface, token *trakt.Token) *CleanupService {
	return &CleanupService{
		repo:             repo,
		allDebridService: allDebrid,
		token:            token,
		watchedDays:      5, // default
	}
}

// SetWatchedDays sets the number of days to look back for watched items
func (s *CleanupService) SetWatchedDays(days int) {
	s.watchedDays = days
}

// CleanWatched removes media that has been watched recently
func (s *CleanupService) CleanWatched() error {
	return s.CleanWatchedWithContext(context.Background())
}

// CleanWatchedWithContext removes media that has been watched recently with context support
func (s *CleanupService) CleanWatchedWithContext(ctx context.Context) error {
	// Add timeout to prevent infinite loops
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	limit := int64(100)
	params := trakt.ListParams{
		OAuth: s.token.AccessToken,
		Limit: &limit,
	}

	historyParams := &trakt.ListHistoryParams{
		ListParams: params,
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -s.watchedDays),
	}

	log.WithFields(log.Fields{
		"days_back": s.watchedDays,
		"start_at":  time.Now().AddDate(0, 0, -s.watchedDays).Format("2006-01-02"),
		"end_at":    time.Now().Format("2006-01-02"),
	}).Info("starting cleanup of watched media")

	iterator := sync.History(historyParams)

	// Limit to 5 pages maximum to prevent pagination issues
	iterator.PageLimit(5)
	log.Debug("set page limit to 5 pages")

	// Track unique items to avoid processing duplicates
	processedItems := make(map[string]bool)
	var cleanedCount int

	for iterator.Next() {
		// Check for context cancellation
		if err := utils.CheckContextCancellation(ctx); err != nil {
			return err
		}

		item, err := iterator.History()
		if err != nil {
			log.WithError(err).Debug("failed to scan watch history item")
			continue
		}

		// Create a unique key for each item
		var itemKey string
		switch string(item.Type) {
		case "movie":
			itemKey = fmt.Sprintf("movie-%d", item.Movie.Trakt)
		case "episode":
			itemKey = fmt.Sprintf("episode-%d", item.Episode.Trakt)
		default:
			continue
		}

		// Skip if already processed
		if processedItems[itemKey] {
			continue
		}
		processedItems[itemKey] = true

		if err := s.processWatchedItem(item); err != nil {
			log.WithError(err).WithField("type", string(item.Type)).Debug("failed to process watched item")
			continue
		}

		cleanedCount++
	}

	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating watch history: %w", err)
	}

	log.WithFields(log.Fields{
		"cleaned_count": cleanedCount,
		"unique_items":  len(processedItems),
		"days_back":     s.watchedDays,
	}).Info("successfully cleaned watched media")

	return nil
}

// processWatchedItem processes a single watched item
func (s *CleanupService) processWatchedItem(item *trakt.History) error {
	switch string(item.Type) {
	case "movie":
		return s.removeMedia(int64(item.Movie.Trakt), item.Movie.Title, models.MediaTypeMovie, 0, 0)
	case "episode":
		return s.removeMedia(int64(item.Episode.Trakt), item.Episode.Title, models.MediaTypeEpisode,
			int(item.Episode.Season), int(item.Episode.Number))
	default:
		log.WithField("type", string(item.Type)).Debug("Ignoring unknown media type")
		return nil
	}
}

// removeMedia removes media and associated data
func (s *CleanupService) removeMedia(traktID int64, title string, mediaType models.MediaType, season, episode int) error {
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		log.WithFields(log.Fields{
			"trakt_id": traktID,
			"title":    title,
		}).Debug("media not found in database, may have already been cleaned up")
		return nil // Not an error - media might have already been cleaned up
	}

	// Check if this is part of a season pack
	if mediaType == models.MediaTypeEpisode && season > 0 && episode > 0 {
		if err := s.handleSeasonPackEpisode(media, season, episode); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"trakt_id": traktID,
				"season":   season,
				"episode":  episode,
			}).Error("failed to handle season pack episode")
		}
	}

	// Remove physical file if it exists
	if err := s.removePhysicalFile(media); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"trakt_id": traktID,
			"title":    title,
			"file":     media.File,
		}).Error("failed to remove physical file")
		// Continue with cleanup even if file removal fails
	}

	// Remove torrents and handle AllDebrid cleanup
	if err := s.removeTorrents(traktID); err != nil {
		log.WithError(err).WithField("trakt_id", traktID).Error("failed to remove torrents")
		// Continue with media removal even if torrent cleanup fails
	}

	// Remove media record
	if err := s.repo.RemoveMedia(traktID); err != nil {
		return fmt.Errorf("removing media from database: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt_id":   traktID,
		"title":      title,
		"media_type": mediaType,
		"file":       media.File,
	}).Info("successfully removed watched media")

	return nil
}

// handleSeasonPackEpisode handles cleanup for episodes that are part of a season pack
func (s *CleanupService) handleSeasonPackEpisode(media *models.Media, season, episode int) error {
	// Since torrents are no longer stored in database, season pack tracking is not available
	// Individual episodes will be cleaned up as they are watched
	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"season":   season,
		"episode":  episode,
	}).Debug("Season pack episode tracking not available since torrents not stored in database")
	return nil
}

// removeTorrents is no longer needed since torrents are not stored in database
func (s *CleanupService) removeTorrents(traktID int64) error {
	log.WithField("trakt_id", traktID).Debug("removeTorrents called but no-op since torrents not stored in database")
	return nil
}

// removePhysicalFile removes the physical media file
func (s *CleanupService) removePhysicalFile(media *models.Media) error {
	if media.File == "" {
		log.WithField("trakt_id", media.Trakt).Debug("no file path to remove")
		return nil
	}

	// Check if file exists before trying to remove
	if _, err := os.Stat(media.File); os.IsNotExist(err) {
		log.WithFields(log.Fields{
			"trakt_id": media.Trakt,
			"file":     media.File,
		}).Debug("file does not exist, skipping removal")
		return nil
	}

	if err := os.Remove(media.File); err != nil {
		return fmt.Errorf("removing file %s: %w", media.File, err)
	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"file":     media.File,
	}).Debug("successfully removed physical file")

	return nil
}

// RemoveMediaManually allows manual removal of media
func (s *CleanupService) RemoveMediaManually(traktID int64, reason string) error {
	return s.RemoveMediaManuallyWithContext(context.Background(), traktID, reason)
}

// RemoveMediaManuallyWithContext allows manual removal of media with context support
func (s *CleanupService) RemoveMediaManuallyWithContext(ctx context.Context, traktID int64, reason string) error {
	// Check for context cancellation
	if err := utils.CheckContextCancellation(ctx); err != nil {
		return err
	}

	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("finding media %d: %w", traktID, err)
	}

	mediaType := media.GetType()
	if err := s.removeMedia(traktID, media.Title, mediaType, int(media.Season), int(media.Number)); err != nil {
		return fmt.Errorf("removing media: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt_id": traktID,
		"title":    media.Title,
		"reason":   reason,
	}).Info("manually removed media")

	return nil
}

// GetCleanupStats returns statistics about potential cleanup candidates
func (s *CleanupService) GetCleanupStats() (*CleanupStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	limit := int64(100)
	params := trakt.ListParams{
		OAuth: s.token.AccessToken,
		Limit: &limit,
	}

	historyParams := &trakt.ListHistoryParams{
		ListParams: params,
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -s.watchedDays),
	}

	iterator := sync.History(historyParams)

	// Limit to 5 pages for stats as well
	iterator.PageLimit(5)

	stats := &CleanupStats{
		WatchedDays: s.watchedDays,
	}

	// Track unique items only
	uniqueItems := make(map[string]bool)

	for iterator.Next() {
		// Check for context cancellation
		if err := utils.CheckContextCancellation(ctx); err != nil {
			return stats, err
		}

		item, err := iterator.History()
		if err != nil {
			continue
		}

		// Create unique key
		var itemKey string
		switch string(item.Type) {
		case "movie":
			itemKey = fmt.Sprintf("movie-%d", item.Movie.Trakt)
			if !uniqueItems[itemKey] {
				uniqueItems[itemKey] = true
				stats.Movies++
				stats.Total++
			}
		case "episode":
			itemKey = fmt.Sprintf("episode-%d", item.Episode.Trakt)
			if !uniqueItems[itemKey] {
				uniqueItems[itemKey] = true
				stats.Episodes++
				stats.Total++
			}
		}
	}

	return stats, iterator.Err()
}

// CleanupStats represents cleanup statistics
type CleanupStats struct {
	WatchedDays int `json:"watched_days"`
	Movies      int `json:"movies"`
	Episodes    int `json:"episodes"`
	Total       int `json:"total"`
}

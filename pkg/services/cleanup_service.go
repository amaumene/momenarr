package services

import (
	"fmt"
	"os"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

const (
	defaultWatchedDays = 5
)

// CleanupService handles cleanup of watched media
type CleanupService struct {
	repo        repository.Repository
	token       *trakt.Token
	watchedDays int
}

// NewCleanupService creates a new CleanupService
func NewCleanupService(repo repository.Repository, token *trakt.Token) *CleanupService {
	return &CleanupService{
		repo:        repo,
		token:       token,
		watchedDays: defaultWatchedDays,
	}
}

// SetWatchedDays sets the number of days to look back for watched items
func (s *CleanupService) SetWatchedDays(days int) {
	s.watchedDays = days
}

// CleanWatched removes media that has been watched recently
func (s *CleanupService) CleanWatched() error {
	params := trakt.ListParams{OAuth: s.token.AccessToken}

	historyParams := &trakt.ListHistoryParams{
		ListParams: params,
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -s.watchedDays),
	}

	iterator := sync.History(historyParams)
	var cleanedCount int

	for iterator.Next() {
		item, err := iterator.History()
		if err != nil {
			log.WithError(err).Error("Failed to scan watch history item")
			continue
		}

		if err := s.processWatchedItem(item); err != nil {
			log.WithError(err).WithField("type", string(item.Type)).Error("Failed to process watched item")
			continue
		}

		cleanedCount++
	}

	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating watch history: %w", err)
	}

	log.WithFields(log.Fields{
		"cleaned_count": cleanedCount,
		"days_back":     s.watchedDays,
	}).Info("Successfully cleaned watched media")

	return nil
}

// processWatchedItem processes a single watched item
func (s *CleanupService) processWatchedItem(item *trakt.History) error {
	switch string(item.Type) {
	case "movie":
		return s.removeMedia(int64(item.Movie.Trakt), item.Movie.Title, models.MediaTypeMovie)
	case "episode":
		return s.removeMedia(int64(item.Episode.Trakt), item.Episode.Title, models.MediaTypeEpisode)
	default:
		log.WithField("type", string(item.Type)).Debug("Ignoring unknown media type")
		return nil
	}
}

// removeMedia removes media and associated data
func (s *CleanupService) removeMedia(traktID int64, title string, mediaType models.MediaType) error {
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("finding media %d (%s): %w", traktID, title, err)
	}

	// Remove physical file if it exists
	if err := s.removePhysicalFile(media); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"trakt": traktID,
			"title": title,
			"file":  media.File,
		}).Error("Failed to remove physical file")
		// Continue with database cleanup even if file removal fails
	}

	// Remove NZB records
	if err := s.removeNZBRecords(traktID); err != nil {
		log.WithError(err).WithField("trakt", traktID).Error("Failed to remove NZB records")
		// Continue with media removal even if NZB cleanup fails
	}

	// Remove media record
	if err := s.repo.RemoveMedia(traktID); err != nil {
		return fmt.Errorf("removing media from database: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt":      traktID,
		"title":      title,
		"media_type": mediaType,
		"file":       media.File,
	}).Info("Successfully removed watched media")

	return nil
}

// removePhysicalFile removes the physical media file
func (s *CleanupService) removePhysicalFile(media *models.Media) error {
	if media.File == "" {
		log.WithField("trakt", media.Trakt).Debug("No file path to remove")
		return nil
	}

	// Check if file exists before trying to remove
	if _, err := os.Stat(media.File); os.IsNotExist(err) {
		log.WithFields(log.Fields{
			"trakt": media.Trakt,
			"file":  media.File,
		}).Debug("File does not exist, skipping removal")
		return nil
	}

	if err := os.Remove(media.File); err != nil {
		return fmt.Errorf("removing file %s: %w", media.File, err)
	}

	log.WithFields(log.Fields{
		"trakt": media.Trakt,
		"file":  media.File,
	}).Debug("Successfully removed physical file")

	return nil
}

// removeNZBRecords removes associated NZB records
func (s *CleanupService) removeNZBRecords(traktID int64) error {
	if err := s.repo.RemoveNZBsByTraktID(traktID); err != nil {
		return fmt.Errorf("removing NZBs for Trakt ID %d: %w", traktID, err)
	}

	log.WithField("trakt", traktID).Debug("Successfully removed NZB records")
	return nil
}

// RemoveMediaManually allows manual removal of media
func (s *CleanupService) RemoveMediaManually(traktID int64, reason string) error {
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("finding media %d: %w", traktID, err)
	}

	mediaType := media.GetType()
	if err := s.removeMedia(traktID, media.Title, mediaType); err != nil {
		return fmt.Errorf("removing media: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt":  traktID,
		"title":  media.Title,
		"reason": reason,
	}).Info("Manually removed media")

	return nil
}

// GetCleanupStats returns statistics about potential cleanup candidates
func (s *CleanupService) GetCleanupStats() (*CleanupStats, error) {
	params := trakt.ListParams{OAuth: s.token.AccessToken}

	historyParams := &trakt.ListHistoryParams{
		ListParams: params,
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -s.watchedDays),
	}

	iterator := sync.History(historyParams)
	stats := &CleanupStats{
		WatchedDays: s.watchedDays,
	}

	for iterator.Next() {
		item, err := iterator.History()
		if err != nil {
			continue
		}

		switch item.Type.String() {
		case "movie":
			stats.Movies++
		case "episode":
			stats.Episodes++
		}
		stats.Total++
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

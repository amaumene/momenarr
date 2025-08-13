package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/amaumene/gostremiofr/pkg/alldebrid"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/utils"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

const (
	defaultWatchedDays   = 5
	cleanupTimeout       = 5 * time.Minute
	historyPageLimit     = 1
	deleteRequestTimeout = 2 * time.Minute
	rateLimitDelay       = 2 * time.Second
	maxWatchedItems      = 30
)

// CleanupService handles cleanup of watched media with AllDebrid support
type CleanupService struct {
	repo            repository.Repository
	allDebridClient *alldebrid.Client
	apiKey          string
	token           *trakt.Token
	watchedDays     int
	rateLimiter     *utils.RateLimiter
}

// CreateCleanupService creates a cleanup service
func CreateCleanupService(repo repository.Repository, allDebridClient *alldebrid.Client, apiKey string, token *trakt.Token) *CleanupService {
	return &CleanupService{
		repo:            repo,
		allDebridClient: allDebridClient,
		apiKey:          apiKey,
		token:           token,
		watchedDays:     defaultWatchedDays,
		rateLimiter:     utils.TraktRateLimiter(),
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
	ctx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	// Use enhanced cleanup logic
	return s.ProcessWatchedMediaEnhanced(ctx)
}

// ProcessWatchedMediaEnhanced processes watched media with season pack awareness
func (s *CleanupService) ProcessWatchedMediaEnhanced(ctx context.Context) error {
	// Get watched history
	iterator := s.createHistoryIterator()
	s.logCleanupStart()

	// Collect all watched items first
	watchedItems := make(map[string]*WatchedItem)
	seasonWatchStatus := make(map[string]*models.SeasonWatchStatus)

	for iterator.Next() {
		item, err := iterator.History()
		if err != nil {
			continue
		}

		if err := s.collectWatchedItem(item, watchedItems, seasonWatchStatus); err != nil {
			log.WithError(err).Debug("Failed to collect watched item")
			continue
		}
	}

	// Process deletions based on type
	return s.processAllDeletions(ctx, watchedItems, seasonWatchStatus)
}

// WatchedItem represents a watched media item
type WatchedItem struct {
	TraktID    int64
	TMDBID     int64
	Title      string
	MediaType  models.MediaType
	Season     int64
	Episode    int64
	WatchedAt  time.Time
	ShowTitle  string
	ShowTMDBID int64
}

// collectWatchedItem collects watched item information
func (s *CleanupService) collectWatchedItem(item *trakt.History, watchedItems map[string]*WatchedItem, seasonWatchStatus map[string]*models.SeasonWatchStatus) error {
	switch string(item.Type) {
	case "movie":
		s.collectWatchedMovie(item, watchedItems)
	case "episode":
		s.collectWatchedEpisode(item, watchedItems, seasonWatchStatus)
	}
	return nil
}

func (s *CleanupService) collectWatchedMovie(item *trakt.History, watchedItems map[string]*WatchedItem) {
	key := fmt.Sprintf("movie_%d", item.Movie.Trakt)
	watchedItems[key] = &WatchedItem{
		TraktID:   int64(item.Movie.Trakt),
		TMDBID:    int64(item.Movie.MediaIDs.TMDB),
		Title:     item.Movie.Title,
		MediaType: models.MediaTypeMovie,
		WatchedAt: item.WatchedAt,
	}
}

func (s *CleanupService) collectWatchedEpisode(item *trakt.History, watchedItems map[string]*WatchedItem, seasonWatchStatus map[string]*models.SeasonWatchStatus) {
	key := fmt.Sprintf("episode_%d", item.Episode.Trakt)
	watchedItems[key] = &WatchedItem{
		TraktID:    int64(item.Episode.Trakt),
		Title:      item.Episode.Title,
		MediaType:  models.MediaTypeEpisode,
		Season:     item.Episode.Season,
		Episode:    item.Episode.Number,
		WatchedAt:  item.WatchedAt,
		ShowTitle:  item.Show.Title,
		ShowTMDBID: int64(item.Show.MediaIDs.TMDB),
	}

	s.updateSeasonWatchStatus(item, seasonWatchStatus)
}

func (s *CleanupService) updateSeasonWatchStatus(item *trakt.History, seasonWatchStatus map[string]*models.SeasonWatchStatus) {
	seasonKey := fmt.Sprintf("%d_S%d", item.Show.MediaIDs.TMDB, item.Episode.Season)

	if _, exists := seasonWatchStatus[seasonKey]; !exists {
		seasonWatchStatus[seasonKey] = &models.SeasonWatchStatus{
			ShowTMDBID:  int64(item.Show.MediaIDs.TMDB),
			ShowTitle:   item.Show.Title,
			Season:      item.Episode.Season,
			WatchedList: []int64{},
		}
	}

	status := seasonWatchStatus[seasonKey]
	status.WatchedList = append(status.WatchedList, item.Episode.Number)
	status.WatchedEpisodes = len(status.WatchedList)
	if item.WatchedAt.After(status.LastWatchedAt) {
		status.LastWatchedAt = item.WatchedAt
	}
}

// processAllDeletions processes all types of deletions
func (s *CleanupService) processAllDeletions(ctx context.Context, watchedItems map[string]*WatchedItem, seasonStatus map[string]*models.SeasonWatchStatus) error {
	s.checkAllSeasons(ctx, seasonStatus)

	deletedCount := 0
	deletedCount += s.deleteMovies(watchedItems)
	deletedCount += s.deleteCompleteSeasons(seasonStatus, watchedItems)
	deletedCount += s.deleteRemainingEpisodes(watchedItems, seasonStatus)

	log.WithField("deleted_count", deletedCount).Info("Completed cleanup of watched media")
	return nil
}

func (s *CleanupService) checkAllSeasons(ctx context.Context, seasonStatus map[string]*models.SeasonWatchStatus) {
	for _, status := range seasonStatus {
		if err := s.checkSeasonCompletion(ctx, status); err != nil {
			log.WithError(err).Debug("Failed to check season completion")
		}
	}
}

func (s *CleanupService) deleteMovies(watchedItems map[string]*WatchedItem) int {
	deletedCount := 0
	for key, item := range watchedItems {
		if item.MediaType == models.MediaTypeMovie {
			if err := s.deleteWatchedMedia(item); err != nil {
				log.WithError(err).WithField("title", item.Title).Error("Failed to delete movie")
			} else {
				deletedCount++
				delete(watchedItems, key)
			}
		}
	}
	return deletedCount
}

func (s *CleanupService) deleteCompleteSeasons(seasonStatus map[string]*models.SeasonWatchStatus, watchedItems map[string]*WatchedItem) int {
	deletedCount := 0
	for seasonKey, status := range seasonStatus {
		if status.IsComplete {
			if err := s.deleteCompleteSeason(status, watchedItems); err != nil {
				log.WithError(err).WithField("season", seasonKey).Error("Failed to delete season")
			} else {
				deletedCount += status.WatchedEpisodes
			}
		}
	}
	return deletedCount
}

func (s *CleanupService) deleteRemainingEpisodes(watchedItems map[string]*WatchedItem, seasonStatus map[string]*models.SeasonWatchStatus) int {
	deletedCount := 0
	for _, item := range watchedItems {
		if item.MediaType != models.MediaTypeEpisode {
			continue
		}

		seasonKey := fmt.Sprintf("%d_S%d", item.ShowTMDBID, item.Season)
		if status, exists := seasonStatus[seasonKey]; exists && status.IsComplete {
			continue
		}

		if err := s.deleteWatchedMedia(item); err != nil {
			log.WithError(err).WithField("episode", item.Title).Error("Failed to delete episode")
		} else {
			deletedCount++
		}
	}
	return deletedCount
}

// createHistoryIterator creates the history iterator
func (s *CleanupService) createHistoryIterator() *trakt.HistoryIterator {
	limit := int64(maxWatchedItems)
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
	iterator.PageLimit(historyPageLimit)
	log.WithField("page_limit", historyPageLimit).Debug("set page limit")
	return iterator
}

// logCleanupStart logs the cleanup start
func (s *CleanupService) logCleanupStart() {
	log.WithFields(log.Fields{
		"days_back": s.watchedDays,
		"start_at":  time.Now().AddDate(0, 0, -s.watchedDays).Format("2006-01-02"),
		"end_at":    time.Now().Format("2006-01-02"),
		"max_items": maxWatchedItems,
	}).Info("starting cleanup of watched media")
}

// RemoveMediaManually allows manual removal of media
func (s *CleanupService) RemoveMediaManually(traktID int64, reason string) error {
	return s.RemoveMediaManuallyWithContext(context.Background(), traktID, reason)
}

// RemoveMediaManuallyWithContext allows manual removal of media with context support
func (s *CleanupService) RemoveMediaManuallyWithContext(ctx context.Context, traktID int64, reason string) error {
	if err := utils.CheckContextCancellation(ctx); err != nil {
		return err
	}

	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("finding media %d: %w", traktID, err)
	}

	watchedItem := &WatchedItem{
		TraktID:   media.Trakt,
		Title:     media.Title,
		MediaType: media.GetType(),
	}

	if err := s.deleteWatchedMedia(watchedItem); err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), deleteRequestTimeout)
	defer cancel()

	iterator := s.createHistoryIterator()
	stats := s.initializeStats()
	uniqueItems := make(map[string]bool)

	if err := s.collectStats(ctx, iterator, stats, uniqueItems); err != nil {
		return stats, err
	}

	return stats, iterator.Err()
}

// initializeStats creates initial stats structure
func (s *CleanupService) initializeStats() *CleanupStats {
	return &CleanupStats{
		WatchedDays: s.watchedDays,
	}
}

// collectStats collects statistics from history
func (s *CleanupService) collectStats(ctx context.Context, iterator *trakt.HistoryIterator,
	stats *CleanupStats, uniqueItems map[string]bool) error {
	for iterator.Next() {
		if err := utils.CheckContextCancellation(ctx); err != nil {
			return err
		}

		if err := s.processStatsItem(iterator, stats, uniqueItems); err != nil {
			continue
		}
	}
	return nil
}

// processStatsItem processes a single item for statistics
func (s *CleanupService) processStatsItem(iterator *trakt.HistoryIterator,
	stats *CleanupStats, uniqueItems map[string]bool) error {
	item, err := iterator.History()
	if err != nil {
		return err
	}

	var itemKey string
	switch string(item.Type) {
	case "movie":
		itemKey = fmt.Sprintf("movie-%d", item.Movie.Trakt)
	case "episode":
		itemKey = fmt.Sprintf("episode-%d", item.Episode.Trakt)
	default:
		return nil
	}

	if itemKey == "" || uniqueItems[itemKey] {
		return nil
	}

	uniqueItems[itemKey] = true
	s.updateStatsForItem(item, stats)
	return nil
}

// updateStatsForItem updates statistics based on item type
func (s *CleanupService) updateStatsForItem(item *trakt.History, stats *CleanupStats) {
	switch string(item.Type) {
	case "movie":
		stats.Movies++
		stats.Total++
	case "episode":
		stats.Episodes++
		stats.Total++
	}
}

// CleanupStats represents cleanup statistics
type CleanupStats struct {
	WatchedDays int `json:"watched_days"`
	Movies      int `json:"movies"`
	Episodes    int `json:"episodes"`
	Total       int `json:"total"`
}

// checkSeasonCompletion checks if a season is fully watched
func (s *CleanupService) checkSeasonCompletion(ctx context.Context, status *models.SeasonWatchStatus) error {
	showTraktID, err := s.getShowTraktID(status.ShowTMDBID)
	if err != nil {
		return err
	}

	seasonInfo, err := s.getSeasonInfo(showTraktID, status.Season)
	if err != nil {
		return err
	}

	status.TotalEpisodes = len(seasonInfo)
	status.IsComplete = status.WatchedEpisodes >= status.TotalEpisodes

	if status.IsComplete {
		log.WithFields(log.Fields{
			"show":     status.ShowTitle,
			"season":   status.Season,
			"episodes": fmt.Sprintf("%d/%d", status.WatchedEpisodes, status.TotalEpisodes),
		}).Info("Season fully watched - eligible for cleanup")
	}

	return nil
}

// getShowTraktID gets Trakt ID from TMDB ID (cached in DB)
func (s *CleanupService) getShowTraktID(tmdbID int64) (int64, error) {
	media, err := s.repo.GetMediaByTMDBAndSeason(tmdbID, 1)
	if err == nil && len(media) > 0 {
		return media[0].Trakt, nil
	}
	return 0, fmt.Errorf("show not found in database")
}

// getSeasonInfo gets season episode information (cached)
func (s *CleanupService) getSeasonInfo(showTraktID int64, season int64) ([]interface{}, error) {
	episodes, err := s.repo.GetEpisodesBySeason(showTraktID, season)
	if err == nil && len(episodes) > 0 {
		result := make([]interface{}, len(episodes))
		for i := range episodes {
			result[i] = episodes[i]
		}
		return result, nil
	}
	return nil, fmt.Errorf("season info not found")
}

// deleteWatchedMedia deletes a single watched media item
func (s *CleanupService) deleteWatchedMedia(item *WatchedItem) error {
	media, err := s.repo.GetMedia(item.TraktID)
	if err != nil {
		return nil
	}

	if media.MagnetID != "" {
		magnetID, err := strconv.ParseInt(media.MagnetID, 10, 64)
		if err == nil {
			if err := s.allDebridClient.DeleteMagnet(s.apiKey, strconv.FormatInt(magnetID, 10)); err != nil {
				log.WithError(err).WithField("magnet_id", media.MagnetID).Warn("Failed to delete from AllDebrid")
			} else {
				log.WithFields(log.Fields{
					"title":     item.Title,
					"magnet_id": media.MagnetID,
				}).Info("Deleted from AllDebrid")
			}
		}
	}

	if err := s.repo.RemoveMedia(item.TraktID); err != nil {
		return fmt.Errorf("removing from database: %w", err)
	}

	log.WithFields(log.Fields{
		"title": item.Title,
		"type":  item.MediaType,
	}).Info("Deleted watched media")

	return nil
}

// deleteCompleteSeason deletes all episodes in a complete season
func (s *CleanupService) deleteCompleteSeason(status *models.SeasonWatchStatus, watchedItems map[string]*WatchedItem) error {
	s.deleteSeasonPackIfExists(status)

	episodes, err := s.repo.GetEpisodesBySeason(status.ShowTMDBID, status.Season)
	if err != nil {
		return err
	}

	s.deleteSeasonEpisodes(episodes, watchedItems)

	log.WithFields(log.Fields{
		"show":     status.ShowTitle,
		"season":   status.Season,
		"episodes": status.WatchedEpisodes,
	}).Info("Deleted complete season")

	return nil
}

func (s *CleanupService) deleteSeasonPackIfExists(status *models.SeasonWatchStatus) {
	seasonPack, err := s.repo.GetSeasonPack(status.ShowTMDBID, status.Season)
	if err != nil || seasonPack == nil {
		return
	}

	s.deleteSeasonPackMagnet(seasonPack, status)

	if err := s.repo.RemoveSeasonPack(seasonPack.ID); err != nil {
		log.WithError(err).Warn("Failed to delete season pack record")
	}
}

func (s *CleanupService) deleteSeasonPackMagnet(seasonPack *models.SeasonPack, status *models.SeasonWatchStatus) {
	if seasonPack.MagnetID == "" {
		return
	}

	magnetID, err := strconv.ParseInt(seasonPack.MagnetID, 10, 64)
	if err != nil {
		return
	}

	if err := s.allDebridClient.DeleteMagnet(s.apiKey, strconv.FormatInt(magnetID, 10)); err != nil {
		log.WithError(err).Warn("Failed to delete season pack from AllDebrid")
	} else {
		log.WithFields(log.Fields{
			"show":      status.ShowTitle,
			"season":    status.Season,
			"magnet_id": seasonPack.MagnetID,
		}).Info("Deleted season pack from AllDebrid")
	}
}

func (s *CleanupService) deleteSeasonEpisodes(episodes []*models.Media, watchedItems map[string]*WatchedItem) {
	for _, ep := range episodes {
		s.deleteEpisodeMagnet(ep)

		if err := s.repo.RemoveMedia(ep.Trakt); err != nil {
			log.WithError(err).Warn("Failed to remove episode from database")
		}

		key := fmt.Sprintf("episode_%d", ep.Trakt)
		delete(watchedItems, key)
	}
}

func (s *CleanupService) deleteEpisodeMagnet(ep *models.Media) {
	if ep.MagnetID == "" || ep.IsSeasonPack {
		return
	}

	magnetID, err := strconv.ParseInt(ep.MagnetID, 10, 64)
	if err != nil {
		return
	}

	if err := s.allDebridClient.DeleteMagnet(s.apiKey, strconv.FormatInt(magnetID, 10)); err != nil {
		log.WithError(err).Warn("Failed to delete episode from AllDebrid")
	}
}

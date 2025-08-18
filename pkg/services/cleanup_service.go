package services

import (
	"context"
	"fmt"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/premiumize"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

const (
	defaultWatchedDays   = 5
	cleanupTimeout       = 5 * time.Minute
	historyPageLimit     = 1
	maxWatchedItems      = 30
)

type CleanupService struct {
	repo             repository.Repository
	premiumizeClient *premiumize.Client
	token            *trakt.Token
	watchedDays      int
}

func NewCleanupService(repo repository.Repository, premiumizeClient *premiumize.Client, token *trakt.Token) *CleanupService {
	return &CleanupService{
		repo:             repo,
		premiumizeClient: premiumizeClient,
		token:            token,
		watchedDays:      defaultWatchedDays,
	}
}

func (s *CleanupService) SetWatchedDays(days int) {
	s.watchedDays = days
}

func (s *CleanupService) CleanWatched() error {
	return s.CleanWatchedWithContext(context.Background())
}

func (s *CleanupService) CleanWatchedWithContext(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	return s.ProcessWatchedMediaEnhanced(ctx)
}

func (s *CleanupService) ProcessWatchedMediaEnhanced(ctx context.Context) error {
	iterator := s.createHistoryIterator()
	s.logCleanupStart()

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

	return s.processAllDeletions(ctx, watchedItems, seasonWatchStatus)
}

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

func (s *CleanupService) processAllDeletions(ctx context.Context, watchedItems map[string]*WatchedItem, seasonStatus map[string]*models.SeasonWatchStatus) error {
	s.checkAllSeasons(ctx, seasonStatus)

	deletedCount := 0
	deletedCount += s.deleteMovies(ctx, watchedItems)
	deletedCount += s.deleteCompleteSeasons(ctx, seasonStatus, watchedItems)
	deletedCount += s.deleteRemainingEpisodes(ctx, watchedItems, seasonStatus)

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

func (s *CleanupService) checkSeasonCompletion(ctx context.Context, status *models.SeasonWatchStatus) error {
	// Get all episodes for this season
	episodes, err := s.repo.GetEpisodesBySeason(status.ShowTMDBID, status.Season)
	if err != nil {
		return fmt.Errorf("getting episodes for season: %w", err)
	}

	status.TotalEpisodes = len(episodes)
	
	// Check if all episodes have been watched
	if status.WatchedEpisodes >= status.TotalEpisodes && status.TotalEpisodes > 0 {
		status.IsComplete = true
		
		// Check for season pack
		if pack, err := s.repo.GetSeasonPack(status.ShowTMDBID, status.Season); err == nil && pack != nil {
			status.SeasonPackID = pack.ID
		}
	}

	return nil
}

func (s *CleanupService) deleteMovies(ctx context.Context, watchedItems map[string]*WatchedItem) int {
	deletedCount := 0
	for key, item := range watchedItems {
		if item.MediaType == models.MediaTypeMovie {
			if err := s.deleteWatchedMedia(ctx, item); err != nil {
				log.WithError(err).WithField("title", item.Title).Error("Failed to delete movie")
			} else {
				deletedCount++
				delete(watchedItems, key)
			}
		}
	}
	return deletedCount
}

func (s *CleanupService) deleteCompleteSeasons(ctx context.Context, seasonStatus map[string]*models.SeasonWatchStatus, watchedItems map[string]*WatchedItem) int {
	deletedCount := 0
	for seasonKey, status := range seasonStatus {
		if status.IsComplete {
			if err := s.deleteCompleteSeason(ctx, status, watchedItems); err != nil {
				log.WithError(err).WithField("season", seasonKey).Error("Failed to delete season")
			} else {
				deletedCount += status.WatchedEpisodes
			}
		}
	}
	return deletedCount
}

func (s *CleanupService) deleteRemainingEpisodes(ctx context.Context, watchedItems map[string]*WatchedItem, seasonStatus map[string]*models.SeasonWatchStatus) int {
	deletedCount := 0
	for _, item := range watchedItems {
		if item.MediaType != models.MediaTypeEpisode {
			continue
		}

		seasonKey := fmt.Sprintf("%d_S%d", item.ShowTMDBID, item.Season)
		if status, exists := seasonStatus[seasonKey]; exists && status.IsComplete {
			continue
		}

		if err := s.deleteWatchedMedia(ctx, item); err != nil {
			log.WithError(err).WithField("episode", item.Title).Error("Failed to delete episode")
		} else {
			deletedCount++
		}
	}
	return deletedCount
}

func (s *CleanupService) deleteWatchedMedia(ctx context.Context, item *WatchedItem) error {
	media, err := s.repo.GetMedia(item.TraktID)
	if err != nil {
		return nil
	}

	s.deleteMediaTransfer(ctx, media, item.Title)
	return s.removeMediaFromDB(item)
}

func (s *CleanupService) deleteMediaTransfer(ctx context.Context, media *models.Media, title string) {
	if media.TransferID == "" {
		return
	}

	if err := s.premiumizeClient.DeleteTransfer(ctx, media.TransferID); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"transfer_id": media.TransferID,
			"title":       title,
		}).Debug("Failed to delete Premiumize transfer")
	} else {
		log.WithFields(log.Fields{
			"transfer_id": media.TransferID,
			"title":       title,
		}).Debug("Deleted Premiumize transfer")
	}
}

func (s *CleanupService) removeMediaFromDB(item *WatchedItem) error {
	if err := s.repo.RemoveMedia(item.TraktID); err != nil {
		return fmt.Errorf("removing media from database: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt_id": item.TraktID,
		"title":    item.Title,
		"type":     item.MediaType,
	}).Info("Removed watched media from database")

	return nil
}

func (s *CleanupService) deleteCompleteSeason(ctx context.Context, status *models.SeasonWatchStatus, watchedItems map[string]*WatchedItem) error {
	s.deleteSeasonPackIfExists(ctx, status)

	episodes, err := s.repo.GetEpisodesBySeason(status.ShowTMDBID, status.Season)
	if err != nil {
		return err
	}

	s.deleteSeasonEpisodes(ctx, episodes, watchedItems)

	log.WithFields(log.Fields{
		"show":     status.ShowTitle,
		"season":   status.Season,
		"episodes": status.WatchedEpisodes,
	}).Info("Deleted complete season")

	return nil
}

func (s *CleanupService) deleteSeasonPackIfExists(ctx context.Context, status *models.SeasonWatchStatus) {
	seasonPack, err := s.repo.GetSeasonPack(status.ShowTMDBID, status.Season)
	if err != nil || seasonPack == nil {
		return
	}

	if seasonPack.TransferID != "" {
		if err := s.premiumizeClient.DeleteTransfer(ctx, seasonPack.TransferID); err != nil {
			log.WithError(err).WithField("transfer_id", seasonPack.TransferID).Debug("Failed to delete season pack transfer")
		} else {
			log.WithField("transfer_id", seasonPack.TransferID).Info("Deleted season pack transfer")
		}
	}

	if err := s.repo.RemoveSeasonPack(seasonPack.ID); err != nil {
		log.WithError(err).WithField("pack_id", seasonPack.ID).Error("Failed to remove season pack from database")
	}
}

func (s *CleanupService) deleteSeasonEpisodes(ctx context.Context, episodes []*models.Media, watchedItems map[string]*WatchedItem) {
	for _, episode := range episodes {
		key := fmt.Sprintf("episode_%d", episode.Trakt)
		if _, exists := watchedItems[key]; exists {
			delete(watchedItems, key)
		}

		if episode.TransferID != "" && !episode.IsSeasonPack {
			if err := s.premiumizeClient.DeleteTransfer(ctx, episode.TransferID); err != nil {
				log.WithError(err).WithField("transfer_id", episode.TransferID).Debug("Failed to delete episode transfer")
			}
		}

		if err := s.repo.RemoveMedia(episode.Trakt); err != nil {
			log.WithError(err).WithField("trakt_id", episode.Trakt).Debug("Failed to remove episode from database")
		}
	}
}

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

func (s *CleanupService) logCleanupStart() {
	log.WithFields(log.Fields{
		"days_back": s.watchedDays,
		"start_at":  time.Now().AddDate(0, 0, -s.watchedDays).Format("2006-01-02"),
		"end_at":    time.Now().Format("2006-01-02"),
		"max_items": maxWatchedItems,
	}).Info("starting cleanup of watched media")
}

func (s *CleanupService) RemoveMediaManually(traktID int64, reason string) error {
	return s.RemoveMediaManuallyWithContext(context.Background(), traktID, reason)
}

func (s *CleanupService) RemoveMediaManuallyWithContext(ctx context.Context, traktID int64, reason string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("finding media %d: %w", traktID, err)
	}

	watchedItem := s.createWatchedItem(media)
	if err := s.deleteWatchedMedia(ctx, watchedItem); err != nil {
		return fmt.Errorf("removing media: %w", err)
	}

	s.logManualRemoval(traktID, media.Title, reason)
	return nil
}

func (s *CleanupService) createWatchedItem(media *models.Media) *WatchedItem {
	return &WatchedItem{
		TraktID:   media.Trakt,
		Title:     media.Title,
		MediaType: media.GetType(),
	}
}

func (s *CleanupService) logManualRemoval(traktID int64, title, reason string) {
	log.WithFields(log.Fields{
		"trakt_id": traktID,
		"title":    title,
		"reason":   reason,
	}).Info("Manually removed media")
}

func (s *CleanupService) GetCleanupStats() (*CleanupStats, error) {
	stats := &CleanupStats{
		WatchedDays: s.watchedDays,
	}

	mediaList, err := s.repo.FindAllMedia()
	if err != nil {
		return nil, fmt.Errorf("getting media list: %w", err)
	}

	for _, media := range mediaList {
		if media.OnDisk {
			stats.TotalOnDisk++
		}
	}

	return stats, nil
}

type CleanupStats struct {
	WatchedDays int `json:"watched_days"`
	TotalOnDisk int `json:"total_on_disk"`
}
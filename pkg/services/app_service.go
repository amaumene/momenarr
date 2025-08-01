// Package services contains business logic and service layer components
package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/utils"
	"github.com/amaumene/momenarr/trakt"
	log "github.com/sirupsen/logrus"
)

// AppService orchestrates the main application functionality
type AppService struct {
	mu              sync.RWMutex
	repo            repository.Repository
	traktService    *TraktService
	torrentService  *TorrentService
	downloadService *DownloadService
	cleanupService  *CleanupService
}

// CreateAppService creates a new application service instance
func CreateAppService(
	repo repository.Repository,
	traktService *TraktService,
	torrentService *TorrentService,
	downloadService *DownloadService,
	cleanupService *CleanupService,
) *AppService {
	return &AppService{
		repo:            repo,
		traktService:    traktService,
		torrentService:  torrentService,
		downloadService: downloadService,
		cleanupService:  cleanupService,
	}
}

// RunTasks executes all main application tasks
func (s *AppService) RunTasks(ctx context.Context) error {
	log.Info("starting application tasks")
	startTime := time.Now()

	services := s.getServices()

	if _, err := s.syncFromTrakt(ctx); err != nil {
		return utils.WrapServiceError("sync from trakt", err)
	}

	if err := services.torrent.PopulateTorrentsWithContext(ctx); err != nil {
		return utils.WrapServiceError("populate torrent entries", err)
	}

	if err := services.download.DownloadNotOnDiskWithContext(ctx); err != nil {
		return utils.WrapServiceError("download media not on disk", err)
	}

	if err := services.cleanup.CleanWatchedWithContext(ctx); err != nil {
		return utils.WrapServiceError("clean watched media", err)
	}

	duration := time.Since(startTime)
	log.WithField("duration", duration).Info("completed all application tasks successfully")

	return nil
}

// syncFromTrakt handles the Trakt synchronization and cleanup
func (s *AppService) syncFromTrakt(ctx context.Context) ([]int64, error) {
	services := s.getServices()

	merged, err := services.trakt.SyncFromTraktWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("syncing from Trakt: %w", err)
	}

	if len(merged) >= 1 {
		if err := s.cleanupRemovedMedia(ctx, merged); err != nil {
			log.WithError(err).Error("failed to cleanup removed media")
		}
	}

	return merged, nil
}

// cleanupRemovedMedia removes media no longer in the current list
func (s *AppService) cleanupRemovedMedia(ctx context.Context, currentTraktIDs []int64) error {
	currentIDs := createIDLookup(currentTraktIDs)
	removedCount := 0

	err := s.repo.ProcessMediaBatchesWithContext(ctx, 100, func(batch []*models.Media) error {
		if err := utils.CheckContextCancellation(ctx); err != nil {
			return err
		}

		for _, media := range batch {
			if !currentIDs[media.Trakt] {
				if err := s.removeMedia(ctx, media); err != nil {
					continue
				}
				removedCount++
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("processing media batches for cleanup: %w", err)
	}

	if removedCount > 0 {
		log.WithField("count", removedCount).Info("removed media no longer in trakt lists")
	}

	return nil
}

// GetMediaStats returns media statistics
func (s *AppService) GetMediaStats() (*MediaStats, error) {
	stats := &MediaStats{}

	err := s.repo.StreamMedia(func(media *models.Media) error {
		updateMediaStats(stats, media)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("streaming media for stats: %w", err)
	}

	return stats, nil
}

// GetCleanupStats returns cleanup statistics
func (s *AppService) GetCleanupStats() (*CleanupStats, error) {
	return s.cleanupService.GetCleanupStats()
}

// GetTorrentsByTraktID is deprecated
func (s *AppService) GetTorrentsByTraktID(traktID int64) ([]interface{}, error) {
	return nil, fmt.Errorf("torrent database functionality has been removed")
}

// GetAllMedia returns all media items
func (s *AppService) GetAllMedia() ([]*models.Media, error) {
	var mediaList []*models.Media

	err := s.repo.StreamMedia(func(media *models.Media) error {
		mediaList = append(mediaList, media)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("streaming media: %w", err)
	}

	return mediaList, nil
}

// UpdateTraktServices updates Trakt-related services
func (s *AppService) UpdateTraktServices(traktService *TraktService, cleanupService *CleanupService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traktService = traktService
	s.cleanupService = cleanupService
}

// UpdateTraktToken updates the Trakt token
func (s *AppService) UpdateTraktToken(token *trakt.Token, cleanupService *CleanupService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traktService.UpdateToken(token)
	s.cleanupService = cleanupService
}

// RetryDownload retries a failed download
func (s *AppService) RetryDownload(traktID int64) error {
	return s.downloadService.RetryFailedDownload(traktID)
}

// CancelDownload cancels a download
func (s *AppService) CancelDownload(traktID int64) error {
	return s.downloadService.CancelDownload(traktID)
}

// GetDownloadStatus gets the status of a download
func (s *AppService) GetDownloadStatus(traktID int64) (string, error) {
	return s.downloadService.GetDownloadStatus(traktID)
}

// RefreshAll manually triggers a full refresh
func (s *AppService) RefreshAll(ctx context.Context) error {
	return s.RunTasks(ctx)
}

// SearchTorrentsForNotDownloaded syncs and searches torrents
func (s *AppService) SearchTorrentsForNotDownloaded(ctx context.Context) error {
	log.Info("starting trakt sync and torrent search")
	startTime := time.Now()

	_, err := s.syncAndCleanup(ctx)
	if err != nil {
		return err
	}

	if err := utils.CheckContextCancellation(ctx); err != nil {
		return err
	}

	if err := s.searchTorrentsForMissing(ctx); err != nil {
		return err
	}

	duration := time.Since(startTime)
	log.WithField("duration", duration).Info("completed trakt sync and torrent search")

	return nil
}

// Close gracefully shuts down the service
func (s *AppService) Close() error {
	log.Info("shutting down application service")

	done := make(chan error, 1)
	go func() {
		done <- s.repo.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("closing repository: %w", err)
		}
	case <-time.After(5 * time.Second):
		log.Warn("database close timeout reached")
		return fmt.Errorf("database close timeout")
	}

	return nil
}

// MediaStats represents media statistics
type MediaStats struct {
	Total       int `json:"total"`
	OnDisk      int `json:"on_disk"`
	NotOnDisk   int `json:"not_on_disk"`
	Movies      int `json:"movies"`
	Episodes    int `json:"episodes"`
	Downloading int `json:"downloading"`
}

// Helper types and functions

type serviceRefs struct {
	trakt    *TraktService
	torrent  *TorrentService
	download *DownloadService
	cleanup  *CleanupService
}

// getServices returns all service references (thread-safe)
func (s *AppService) getServices() serviceRefs {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return serviceRefs{
		trakt:    s.traktService,
		torrent:  s.torrentService,
		download: s.downloadService,
		cleanup:  s.cleanupService,
	}
}

// createIDLookup creates a map for fast ID lookups
func createIDLookup(ids []int64) map[int64]bool {
	lookup := make(map[int64]bool, len(ids))
	for _, id := range ids {
		lookup[id] = true
	}
	return lookup
}

// updateMediaStats updates statistics for a media item
func updateMediaStats(stats *MediaStats, media *models.Media) {
	stats.Total++
	if media.OnDisk {
		stats.OnDisk++
	} else {
		stats.NotOnDisk++
	}

	if media.IsMovie() {
		stats.Movies++
	} else {
		stats.Episodes++
	}
}

// removeMedia removes a media item with proper logging
func (s *AppService) removeMedia(ctx context.Context, media *models.Media) error {
	services := s.getServices()
	reason := "not in current Trakt lists"

	if err := services.cleanup.RemoveMediaManuallyWithContext(ctx, media.Trakt, reason); err != nil {
		utils.LogMediaOperation("remove media", media).
			WithError(err).
			Error("failed to remove media")
		return err
	}
	return nil
}

// syncAndCleanup performs sync and cleanup operations
func (s *AppService) syncAndCleanup(ctx context.Context) ([]int64, error) {
	log.Info("syncing media from trakt")
	merged, err := s.syncFromTrakt(ctx)
	if err != nil {
		return nil, utils.WrapServiceError("sync from trakt", err)
	}

	log.WithField("synced_count", len(merged)).Info("synced media from trakt")

	if len(merged) >= 1 {
		log.Info("cleaning up removed media")
		if err := s.cleanupRemovedMedia(ctx, merged); err != nil {
			log.WithError(err).Error("failed to cleanup removed media")
		}
	}

	return merged, nil
}

// searchTorrentsForMissing searches torrents for missing media
func (s *AppService) searchTorrentsForMissing(ctx context.Context) error {
	mediaNotOnDisk, err := s.repo.FindMediaNotOnDisk()
	if err != nil {
		log.WithError(err).Error("failed to get media not on disk")
		return fmt.Errorf("getting media not on disk: %w", err)
	}

	log.WithField("count", len(mediaNotOnDisk)).Info("found media not on disk")

	if len(mediaNotOnDisk) == 0 {
		log.Info("no media not on disk found")
		return nil
	}

	services := s.getServices()
	log.Info("searching for torrents")

	if err := services.download.DownloadNotOnDiskWithContext(ctx); err != nil {
		log.WithError(err).Error("failed to search torrents")
		return fmt.Errorf("searching torrents: %w", err)
	}

	return nil
}

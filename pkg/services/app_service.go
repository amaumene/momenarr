package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type NewAppService struct {
	mu              sync.RWMutex
	repo            repository.Repository
	traktService    *TraktService
	torrentService  *TorrentService
	downloadService *NewDownloadService
	cleanupService  *NewCleanupService
}

func CreateNewAppService(
	repo repository.Repository,
	traktService *TraktService,
	torrentService *TorrentService,
	downloadService *NewDownloadService,
	cleanupService *NewCleanupService,
) *NewAppService {
	return &NewAppService{
		repo:            repo,
		traktService:    traktService,
		torrentService:  torrentService,
		downloadService: downloadService,
		cleanupService:  cleanupService,
	}
}

// RunTasks executes all main application tasks with proper synchronization
func (s *NewAppService) RunTasks(ctx context.Context) error {
	log.Info("starting application tasks")
	startTime := time.Now()

	// Get all service references at once to minimize lock time
	s.mu.RLock()
	torrentService := s.torrentService
	downloadService := s.downloadService
	cleanupService := s.cleanupService
	s.mu.RUnlock()

	if _, err := s.syncFromTrakt(ctx); err != nil {
		return utils.WrapServiceError("sync from trakt", err)
	}

	if err := torrentService.PopulateTorrentsWithContext(ctx); err != nil {
		return utils.WrapServiceError("populate torrent entries", err)
	}

	if err := downloadService.DownloadNotOnDiskWithContext(ctx); err != nil {
		return utils.WrapServiceError("download media not on disk", err)
	}

	if err := cleanupService.CleanWatchedWithContext(ctx); err != nil {
		return utils.WrapServiceError("clean watched media", err)
	}

	duration := time.Since(startTime)
	log.WithField("duration", duration).Info("completed all application tasks successfully")

	return nil
}

// syncFromTrakt handles the Trakt synchronization and cleanup
func (s *NewAppService) syncFromTrakt(ctx context.Context) ([]int64, error) {
	s.mu.RLock()
	traktService := s.traktService
	s.mu.RUnlock()

	merged, err := traktService.SyncFromTraktWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("syncing from Trakt: %w", err)
	}

	if len(merged) >= 1 {
		if err := s.cleanupRemovedMedia(ctx, merged); err != nil {
			log.WithError(err).Error("failed to cleanup removed media")
			// Don't return error as sync was successful
		}
	}

	return merged, nil
}

// cleanupRemovedMedia removes media that is no longer in the merged list using streaming
func (s *NewAppService) cleanupRemovedMedia(ctx context.Context, currentTraktIDs []int64) error {
	// Create a map for faster lookup
	currentIDs := make(map[int64]bool, len(currentTraktIDs))
	for _, id := range currentTraktIDs {
		currentIDs[id] = true
	}

	var removedCount int

	err := s.repo.ProcessMediaBatchesWithContext(ctx, 100, func(batch []*models.Media) error {
		// Check context cancellation
		if err := utils.CheckContextCancellation(ctx); err != nil {
			return err
		}

		for _, media := range batch {
			if !currentIDs[media.Trakt] {
				if err := s.cleanupService.RemoveMediaManuallyWithContext(ctx, media.Trakt, "not in current Trakt lists"); err != nil {
					utils.LogMediaOperation("remove media not in current lists", media).WithError(err).Error("failed to remove media not in current lists")
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

func (s *NewAppService) GetMediaStats() (*MediaStats, error) {
	stats := &MediaStats{}

	// Use streaming to avoid loading all media into memory
	err := s.repo.StreamMedia(func(media *models.Media) error {
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

		// Check if downloading by looking for associated torrents
		torrents, _ := s.repo.FindAllTorrentsByTraktID(media.Trakt)
		for _, torrent := range torrents {
			if torrent.AllDebridID > 0 && !torrent.Failed {
				stats.Downloading++
				break
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("streaming media for stats: %w", err)
	}

	return stats, nil
}

func (s *NewAppService) GetCleanupStats() (*CleanupStats, error) {
	return s.cleanupService.GetCleanupStats()
}

func (s *NewAppService) GetTorrentsByTraktID(traktID int64) ([]*models.Torrent, error) {
	return s.repo.FindAllTorrentsByTraktID(traktID)
}

// GetAllMedia returns all media items for display
func (s *NewAppService) GetAllMedia() ([]*models.Media, error) {
	var mediaList []*models.Media

	// Use streaming to avoid loading all media into memory
	err := s.repo.StreamMedia(func(media *models.Media) error {
		mediaList = append(mediaList, media)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("streaming media: %w", err)
	}

	return mediaList, nil
}

// UpdateTraktServices updates the Trakt-related services with new token (thread-safe)
func (s *NewAppService) UpdateTraktServices(traktService *TraktService, cleanupService *NewCleanupService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traktService = traktService
	s.cleanupService = cleanupService
}

// RetryDownload retries a failed download
func (s *NewAppService) RetryDownload(traktID int64) error {
	return s.downloadService.RetryFailedDownload(traktID)
}

// CancelDownload cancels a download
func (s *NewAppService) CancelDownload(traktID int64) error {
	return s.downloadService.CancelDownload(traktID)
}

// GetDownloadStatus gets the status of a download
func (s *NewAppService) GetDownloadStatus(traktID int64) (string, error) {
	return s.downloadService.GetDownloadStatus(traktID)
}

// RefreshAll manually triggers a full refresh
func (s *NewAppService) RefreshAll(ctx context.Context) error {
	return s.RunTasks(ctx)
}

// SearchTorrentsForNotDownloaded syncs with Trakt and searches for torrents for media not marked as downloaded
func (s *NewAppService) SearchTorrentsForNotDownloaded(ctx context.Context) error {
	log.Info("starting trakt sync and torrent search for media not on disk")
	startTime := time.Now()

	// First, sync with Trakt to get the latest media list
	log.Info("syncing media from trakt")
	merged, err := s.syncFromTrakt(ctx)
	if err != nil {
		return utils.WrapServiceError("sync from trakt", err)
	}

	log.WithField("synced_count", len(merged)).Info("successfully synced media from trakt")

	// Cleanup removed media if we have a reasonable amount of synced media
	if len(merged) >= 1 {
		log.Info("cleaning up media no longer in trakt lists")
		if err := s.cleanupRemovedMedia(ctx, merged); err != nil {
			log.WithError(err).Error("failed to cleanup removed media")
			// Don't return error as sync was successful
		}
	}

	// Check context after Trakt sync
	if err := utils.CheckContextCancellation(ctx); err != nil {
		return err
	}

	// Get count of media not on disk after sync
	mediaNotOnDisk, err := s.repo.FindMediaNotOnDisk()
	if err != nil {
		log.WithError(err).Error("failed to get count of media not on disk")
		return fmt.Errorf("getting media not on disk: %w", err)
	}

	log.WithField("media_count", len(mediaNotOnDisk)).Info("found media not on disk to search torrents for")

	if len(mediaNotOnDisk) == 0 {
		log.Info("no media not on disk found, nothing to search for")
		return nil
	}

	// Get all service references at once to minimize lock time
	s.mu.RLock()
	downloadService := s.downloadService
	s.mu.RUnlock()

	// This will search for torrents and attempt to download media not on disk
	log.Info("searching for torrents for media not on disk")
	if err := downloadService.DownloadNotOnDiskWithContext(ctx); err != nil {
		log.WithError(err).Error("failed to search torrents for media not on disk")
		return fmt.Errorf("searching torrents for media not on disk: %w", err)
	}

	duration := time.Since(startTime)
	log.WithField("duration", duration).Info("successfully completed trakt sync and torrent search for media not on disk")

	return nil
}

func (s *NewAppService) Close() error {
	log.Info("shutting down application service")

	// Add timeout for database closing to prevent hanging
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
		log.Warn("database close timeout reached, forcing shutdown")
		return fmt.Errorf("database close timeout after 5 seconds")
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

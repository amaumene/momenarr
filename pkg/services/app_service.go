package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

// AppService coordinates all application services
type AppService struct {
	mu                  sync.RWMutex
	repo                repository.Repository
	traktService        *TraktService
	nzbService          *NZBService
	downloadService     *DownloadService
	notificationService *NotificationService
	cleanupService      *CleanupService
}

func NewAppService(
	repo repository.Repository,
	traktService *TraktService,
	nzbService *NZBService,
	downloadService *DownloadService,
	notificationService *NotificationService,
	cleanupService *CleanupService,
) *AppService {
	return &AppService{
		repo:                repo,
		traktService:        traktService,
		nzbService:          nzbService,
		downloadService:     downloadService,
		notificationService: notificationService,
		cleanupService:      cleanupService,
	}
}

// RunTasks executes all main application tasks with proper synchronization
func (s *AppService) RunTasks(ctx context.Context) error {
	log.Info("Starting application tasks")
	startTime := time.Now()

	// Get all service references at once to minimize lock time
	s.mu.RLock()
	nzbService := s.nzbService
	downloadService := s.downloadService
	cleanupService := s.cleanupService
	s.mu.RUnlock()

	if err := s.syncFromTrakt(ctx); err != nil {
		log.WithError(err).Error("Failed to sync from Trakt")
		return fmt.Errorf("syncing from Trakt: %w", err)
	}

	if err := nzbService.PopulateNZBWithContext(ctx); err != nil {
		log.WithError(err).Error("Failed to populate NZB entries")
		return fmt.Errorf("populating NZB entries: %w", err)
	}

	if err := downloadService.DownloadNotOnDiskWithContext(ctx); err != nil {
		log.WithError(err).Error("Failed to download media not on disk")
		return fmt.Errorf("downloading media not on disk: %w", err)
	}

	if err := cleanupService.CleanWatchedWithContext(ctx); err != nil {
		log.WithError(err).Error("Failed to clean watched media")
		return fmt.Errorf("cleaning watched media: %w", err)
	}

	duration := time.Since(startTime)
	log.WithField("duration", duration).Info("Successfully completed all application tasks")

	return nil
}

// syncFromTrakt handles the Trakt synchronization and cleanup
func (s *AppService) syncFromTrakt(ctx context.Context) error {
	s.mu.RLock()
	traktService := s.traktService
	s.mu.RUnlock()

	merged, err := traktService.SyncFromTraktWithContext(ctx)
	if err != nil {
		return fmt.Errorf("syncing from Trakt: %w", err)
	}

	if len(merged) >= 1 {
		if err := s.cleanupRemovedMedia(ctx, merged); err != nil {
			log.WithError(err).Error("Failed to cleanup removed media")
			// Don't return error as sync was successful
		}
	}

	return nil
}

// cleanupRemovedMedia removes media that is no longer in the merged list using streaming
func (s *AppService) cleanupRemovedMedia(ctx context.Context, currentTraktIDs []int64) error {
	// Create a map for faster lookup
	currentIDs := make(map[int64]bool, len(currentTraktIDs))
	for _, id := range currentTraktIDs {
		currentIDs[id] = true
	}

	var removedCount int

	err := s.repo.ProcessMediaBatchesWithContext(ctx, 100, func(batch []*models.Media) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		for _, media := range batch {
			if !currentIDs[media.Trakt] {
				if err := s.cleanupService.RemoveMediaManuallyWithContext(ctx, media.Trakt, "not in current Trakt lists"); err != nil {
					log.WithError(err).WithFields(log.Fields{
						"trakt": media.Trakt,
						"title": media.Title,
					}).Error("Failed to remove media not in current lists")
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
		log.WithField("count", removedCount).Info("Removed media no longer in Trakt lists")
	}

	return nil
}

// ProcessNotification processes a download notification
func (s *AppService) ProcessNotification(notification *models.Notification) error {
	return s.notificationService.ProcessNotification(notification)
}

// ProcessNotificationWithContext processes a download notification with context
func (s *AppService) ProcessNotificationWithContext(ctx context.Context, notification *models.Notification) error {
	return s.notificationService.ProcessNotificationWithContext(ctx, notification)
}

func (s *AppService) GetMediaStats() (*MediaStats, error) {
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

		if media.DownloadID > 0 {
			stats.Downloading++
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("streaming media for stats: %w", err)
	}

	return stats, nil
}

func (s *AppService) GetCleanupStats() (*CleanupStats, error) {
	return s.cleanupService.GetCleanupStats()
}

func (s *AppService) GetNZBsByTraktID(traktID int64) ([]*models.NZB, error) {
	return s.repo.FindAllNZBsByTraktID(traktID)
}


// GetAllMedia returns all media items for display
func (s *AppService) GetAllMedia() ([]*models.Media, error) {
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
func (s *AppService) UpdateTraktServices(traktService *TraktService, cleanupService *CleanupService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traktService = traktService
	s.cleanupService = cleanupService
}

func (s *AppService) Close() error {
	log.Info("Shutting down application service")

	if err := s.repo.Close(); err != nil {
		return fmt.Errorf("closing repository: %w", err)
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

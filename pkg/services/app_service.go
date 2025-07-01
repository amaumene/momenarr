package services

import (
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

// NewAppService creates a new AppService
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

// isTestMode checks if any service is running in test mode
func (s *AppService) isTestMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Check if download service is in test mode
	if s.downloadService != nil {
		return s.downloadService.IsTestMode()
	}
	return false
}

// RunTasks executes all main application tasks with proper synchronization
func (s *AppService) RunTasks() error {
	log.Info("Starting application tasks")
	startTime := time.Now()

	// 1. Sync from Trakt
	if err := s.syncFromTrakt(); err != nil {
		log.WithError(err).Error("Failed to sync from Trakt")
		return fmt.Errorf("syncing from Trakt: %w", err)
	}

	// 2. Populate NZB entries
	s.mu.RLock()
	nzbService := s.nzbService
	s.mu.RUnlock()
	
	if err := nzbService.PopulateNZB(); err != nil {
		log.WithError(err).Error("Failed to populate NZB entries")
		return fmt.Errorf("populating NZB entries: %w", err)
	}

	// 3. Download media not on disk
	s.mu.RLock()
	downloadService := s.downloadService
	s.mu.RUnlock()
	
	// Skip download processing in test mode since database will be empty
	if s.isTestMode() {
		log.Info("🧪 TEST MODE: Skipping download processing - database contains no NZBs")
	} else {
		if err := downloadService.DownloadNotOnDisk(); err != nil {
			log.WithError(err).Error("Failed to download media not on disk")
			return fmt.Errorf("downloading media not on disk: %w", err)
		}
	}

	// 4. Clean watched media
	s.mu.RLock()
	cleanupService := s.cleanupService
	s.mu.RUnlock()
	
	if err := cleanupService.CleanWatched(); err != nil {
		log.WithError(err).Error("Failed to clean watched media")
		return fmt.Errorf("cleaning watched media: %w", err)
	}

	duration := time.Since(startTime)
	log.WithField("duration", duration).Info("Successfully completed all application tasks")

	return nil
}

// syncFromTrakt handles the Trakt synchronization and cleanup
func (s *AppService) syncFromTrakt() error {
	s.mu.RLock()
	traktService := s.traktService
	s.mu.RUnlock()
	
	merged, err := traktService.SyncFromTrakt()
	if err != nil {
		return fmt.Errorf("syncing from Trakt: %w", err)
	}

	if len(merged) >= 1 {
		if err := s.cleanupRemovedMedia(merged); err != nil {
			log.WithError(err).Error("Failed to cleanup removed media")
			// Don't return error as sync was successful
		}
	}

	return nil
}

// cleanupRemovedMedia removes media that is no longer in the merged list using streaming
func (s *AppService) cleanupRemovedMedia(currentTraktIDs []int64) error {
	// Create a map for faster lookup
	currentIDs := make(map[int64]bool, len(currentTraktIDs))
	for _, id := range currentTraktIDs {
		currentIDs[id] = true
	}

	var removedCount int
	
	// Process media in batches to avoid loading everything into memory
	err := s.repo.ProcessMediaBatches(100, func(batch []*models.Media) error {
		for _, media := range batch {
			if !currentIDs[media.Trakt] {
				if err := s.cleanupService.RemoveMediaManually(media.Trakt, "not in current Trakt lists"); err != nil {
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

// GetMediaStats returns statistics about media in the system
func (s *AppService) GetMediaStats() (*MediaStats, error) {
	allMedia, err := s.repo.FindAllMedia()
	if err != nil {
		return nil, fmt.Errorf("finding all media: %w", err)
	}

	stats := &MediaStats{}
	for _, media := range allMedia {
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
	}

	return stats, nil
}

// GetCleanupStats returns cleanup statistics
func (s *AppService) GetCleanupStats() (*CleanupStats, error) {
	return s.cleanupService.GetCleanupStats()
}

// RetryFailedDownload retries a failed download
func (s *AppService) RetryFailedDownload(traktID int64) error {
	return s.downloadService.RetryFailedDownload(traktID)
}

// CancelDownload cancels an active download
func (s *AppService) CancelDownload(downloadID int64) error {
	return s.downloadService.CancelDownload(downloadID)
}

// GetDownloadStatus gets the status of a download
func (s *AppService) GetDownloadStatus(downloadID int64) (string, error) {
	return s.downloadService.GetDownloadStatus(downloadID)
}


// UpdateTraktServices updates the Trakt-related services with new token (thread-safe)
func (s *AppService) UpdateTraktServices(traktService *TraktService, cleanupService *CleanupService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traktService = traktService
	s.cleanupService = cleanupService
}

// Close gracefully shuts down the application service
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
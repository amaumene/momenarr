package services

import (
	"fmt"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

// AppService coordinates all application services
type AppService struct {
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

// RunTasks executes all main application tasks
func (s *AppService) RunTasks() error {
	log.Info("Starting application tasks")
	startTime := time.Now()

	// 1. Sync from Trakt
	if err := s.syncFromTrakt(); err != nil {
		log.WithError(err).Error("Failed to sync from Trakt")
		return fmt.Errorf("syncing from Trakt: %w", err)
	}

	// 2. Populate NZB entries
	if err := s.nzbService.PopulateNZB(); err != nil {
		log.WithError(err).Error("Failed to populate NZB entries")
		return fmt.Errorf("populating NZB entries: %w", err)
	}

	// 3. Download media not on disk
	if err := s.downloadService.DownloadNotOnDisk(); err != nil {
		log.WithError(err).Error("Failed to download media not on disk")
		return fmt.Errorf("downloading media not on disk: %w", err)
	}

	// 4. Clean watched media
	if err := s.cleanupService.CleanWatched(); err != nil {
		log.WithError(err).Error("Failed to clean watched media")
		return fmt.Errorf("cleaning watched media: %w", err)
	}

	duration := time.Since(startTime)
	log.WithField("duration", duration).Info("Successfully completed all application tasks")

	return nil
}

// syncFromTrakt handles the Trakt synchronization and cleanup
func (s *AppService) syncFromTrakt() error {
	merged, err := s.traktService.SyncFromTrakt()
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

// cleanupRemovedMedia removes media that is no longer in the merged list
func (s *AppService) cleanupRemovedMedia(currentTraktIDs []int64) error {
	allMedia, err := s.repo.FindAllMedia()
	if err != nil {
		return fmt.Errorf("finding all media: %w", err)
	}

	// Create a map for faster lookup
	currentIDs := make(map[int64]bool)
	for _, id := range currentTraktIDs {
		currentIDs[id] = true
	}

	var removedCount int
	for _, media := range allMedia {
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


// UpdateTraktServices updates the Trakt-related services with new token
func (s *AppService) UpdateTraktServices(traktService *TraktService, cleanupService *CleanupService) {
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
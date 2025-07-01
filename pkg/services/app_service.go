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
func (s *AppService) RunTasks() error {
	log.Info("Starting application tasks")
	startTime := time.Now()

	if err := s.syncFromTrakt(); err != nil {
		log.WithError(err).Error("Failed to sync from Trakt")
		return fmt.Errorf("syncing from Trakt: %w", err)
	}

	s.mu.RLock()
	nzbService := s.nzbService
	s.mu.RUnlock()

	if err := nzbService.PopulateNZB(); err != nil {
		log.WithError(err).Error("Failed to populate NZB entries")
		return fmt.Errorf("populating NZB entries: %w", err)
	}

	s.mu.RLock()
	downloadService := s.downloadService
	s.mu.RUnlock()

	if err := downloadService.DownloadNotOnDisk(); err != nil {
		log.WithError(err).Error("Failed to download media not on disk")
		return fmt.Errorf("downloading media not on disk: %w", err)
	}

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

func (s *AppService) GetCleanupStats() (*CleanupStats, error) {
	return s.cleanupService.GetCleanupStats()
}

func (s *AppService) RetryFailedDownload(traktID int64) error {
	return s.downloadService.RetryFailedDownload(traktID)
}

func (s *AppService) CancelDownload(downloadID int64) error {
	return s.downloadService.CancelDownload(downloadID)
}

func (s *AppService) GetDownloadStatus(downloadID int64) (string, error) {
	return s.downloadService.GetDownloadStatus(downloadID)
}

func (s *AppService) GetNZBsByTraktID(traktID int64) ([]*models.NZB, error) {
	return s.repo.FindAllNZBsByTraktID(traktID)
}

// MediaStatusItem represents a media item for status display
type MediaStatusItem struct {
	TraktID  int64  `json:"trakt_id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Season   int64  `json:"season,omitempty"`
	Episode  int64  `json:"episode,omitempty"`
	Year     int64  `json:"year,omitempty"`
	IMDBID   string `json:"imdb_id"`
	OnDisk   bool   `json:"on_disk"`
	Status   string `json:"status"`
	FilePath string `json:"file_path,omitempty"`
}

func (s *AppService) GetMediaStatus() ([]*MediaStatusItem, error) {
	allMedia, err := s.repo.FindAllMedia()
	if err != nil {
		return nil, fmt.Errorf("finding all media: %w", err)
	}

	var statusItems []*MediaStatusItem
	for _, media := range allMedia {
		item := &MediaStatusItem{
			TraktID:  media.Trakt,
			Title:    media.Title,
			Year:     media.Year,
			IMDBID:   media.IMDB,
			OnDisk:   media.OnDisk,
			FilePath: media.File,
		}

		if media.IsEpisode() {
			item.Type = "episode"
			item.Season = media.Season
			item.Episode = media.Number
		} else {
			item.Type = "movie"
		}

		// Set human-readable status
		if media.OnDisk {
			item.Status = "Available"
		} else if media.DownloadID > 0 {
			item.Status = "Downloading"
		} else {
			item.Status = "Wanted"
		}

		statusItems = append(statusItems, item)
	}

	return statusItems, nil
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

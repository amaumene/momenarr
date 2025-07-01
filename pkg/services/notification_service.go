package services

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/amaumene/momenarr/nzbget"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

const (
	categoryMomenarr = "momenarr"
	maxHistoryRetries = 3
	historyRetryDelay = 10 * time.Second
)

// NotificationService handles download notifications
type NotificationService struct {
	repo            repository.Repository
	nzbGet          *nzbget.NZBGet
	downloadService *DownloadService
	downloadDir     string
}

// NewNotificationService creates a new NotificationService
func NewNotificationService(repo repository.Repository, nzbGet *nzbget.NZBGet, downloadService *DownloadService, downloadDir string) *NotificationService {
	return &NotificationService{
		repo:            repo,
		nzbGet:          nzbGet,
		downloadService: downloadService,
		downloadDir:     downloadDir,
	}
}

// ProcessNotification processes a download notification
func (s *NotificationService) ProcessNotification(notification *models.Notification) error {
	if notification.Category != categoryMomenarr {
		log.WithField("category", notification.Category).Debug("Ignoring notification for different category")
		return nil
	}

	traktID, err := notification.GetTraktID()
	if err != nil {
		return fmt.Errorf("parsing Trakt ID: %w", err)
	}

	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("finding media for Trakt ID %d: %w", traktID, err)
	}

	processedNotification := &models.ProcessedNotification{
		Notification: notification,
		TraktID:      traktID,
		ProcessedAt:  time.Now(),
	}

	if notification.IsSuccess() {
		if err := s.handleDownloadSuccess(processedNotification, media); err != nil {
			return fmt.Errorf("handling download success: %w", err)
		}
	} else {
		if err := s.handleDownloadFailure(processedNotification); err != nil {
			return fmt.Errorf("handling download failure: %w", err)
		}
	}

	if err := s.deleteFromHistory(media); err != nil {
		log.WithError(err).WithField("trakt", traktID).Error("Failed to delete from history")
		// Don't return error as this is not critical
	}

	log.WithFields(log.Fields{
		"trakt":  traktID,
		"title":  media.Title,
		"status": notification.Status,
	}).Info("Successfully processed notification")

	return nil
}

// handleDownloadSuccess handles successful download notifications
func (s *NotificationService) handleDownloadSuccess(notification *models.ProcessedNotification, media *models.Media) error {
	// Find the biggest file in the download directory
	biggestFile, err := s.findBiggestFile(notification.Dir)
	if err != nil {
		return fmt.Errorf("finding biggest file in %s: %w", notification.Dir, err)
	}

	// Move file to final destination
	destPath := filepath.Join(s.downloadDir, filepath.Base(biggestFile))
	if err := os.Rename(biggestFile, destPath); err != nil {
		return fmt.Errorf("moving file from %s to %s: %w", biggestFile, destPath, err)
	}

	// Clean up the download directory
	if err := os.RemoveAll(notification.Dir); err != nil {
		log.WithError(err).WithField("dir", notification.Dir).Error("Failed to remove download directory")
		// Don't return error as file has already been moved
	}

	// Update media record
	media.File = destPath
	media.OnDisk = true
	media.UpdatedAt = time.Now()

	if err := s.repo.SaveMedia(media); err != nil {
		return fmt.Errorf("updating media record: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt":     notification.TraktID,
		"title":     media.Title,
		"file_path": destPath,
	}).Info("Successfully processed download success")

	return nil
}

// handleDownloadFailure handles failed download notifications
func (s *NotificationService) handleDownloadFailure(notification *models.ProcessedNotification) error {
	// Retry download with a different NZB
	if err := s.downloadService.RetryFailedDownload(notification.TraktID); err != nil {
		log.WithError(err).WithField("trakt", notification.TraktID).Error("Failed to retry download")
		// Don't return error as the main failure handling is complete
	}

	log.WithFields(log.Fields{
		"trakt": notification.TraktID,
		"title": notification.Name,
	}).Info("Successfully processed download failure")

	return nil
}

// deleteFromHistory deletes a download from NZBGet history
func (s *NotificationService) deleteFromHistory(media *models.Media) error {
	if media.DownloadID == 0 {
		return fmt.Errorf("media has no download ID")
	}

	for i := 0; i < maxHistoryRetries; i++ {
		history, err := s.nzbGet.History(false)
		if err != nil {
			return fmt.Errorf("getting NZBGet history: %w", err)
		}

		for _, item := range history {
			if int64(item.NZBID) == media.DownloadID {
				IDs := []int64{media.DownloadID}
				result, err := s.nzbGet.EditQueue("HistoryFinalDelete", "", IDs)
				if err != nil {
					return fmt.Errorf("deleting from NZBGet history: %w", err)
				}
				if !result {
					return fmt.Errorf("failed to delete from NZBGet history")
				}

				log.WithFields(log.Fields{
					"trakt":       media.Trakt,
					"download_id": media.DownloadID,
				}).Debug("Successfully deleted from NZBGet history")

				return nil
			}
		}

		// If not found, wait and retry
		if i < maxHistoryRetries-1 {
			time.Sleep(historyRetryDelay)
		}
	}

	return fmt.Errorf("download ID %d not found in history after %d retries", media.DownloadID, maxHistoryRetries)
}

// findBiggestFile finds the biggest file in a directory
func (s *NotificationService) findBiggestFile(dir string) (string, error) {
	var biggestFile string
	var maxSize int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && info.Size() > maxSize {
			biggestFile = path
			maxSize = info.Size()
		}
		
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("walking directory %s: %w", dir, err)
	}

	if biggestFile == "" {
		return "", fmt.Errorf("no files found in directory %s", dir)
	}

	log.WithFields(log.Fields{
		"dir":     dir,
		"file":    biggestFile,
		"size_mb": maxSize / (1024 * 1024),
	}).Debug("Found biggest file")

	return biggestFile, nil
}
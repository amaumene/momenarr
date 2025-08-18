package services

import (
	"context"
	"fmt"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/premiumize"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

const (
	categoryMomenarr  = "momenarr"
	maxHistoryRetries = 3
	historyRetryDelay = 10 * time.Second
)

// NotificationService handles download notifications
type NotificationService struct {
	repo             repository.Repository
	premiumizeClient *premiumize.Client
	downloadService  *DownloadService
	downloadDir      string
}

// NewNotificationService creates a new NotificationService
func NewNotificationService(repo repository.Repository, premiumizeClient *premiumize.Client, downloadService *DownloadService, downloadDir string) *NotificationService {
	return &NotificationService{
		repo:             repo,
		premiumizeClient: premiumizeClient,
		downloadService:  downloadService,
		downloadDir:      downloadDir,
	}
}

// ProcessNotification processes a download notification
func (s *NotificationService) ProcessNotification(notification *models.Notification) error {
	return s.ProcessNotificationWithContext(context.Background(), notification)
}

// ProcessNotificationWithContext processes a download notification with context support
func (s *NotificationService) ProcessNotificationWithContext(ctx context.Context, notification *models.Notification) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	traktID, media, err := s.validateNotification(notification)
	if err != nil {
		return err
	}

	processedNotification := &models.ProcessedNotification{
		Notification: notification,
		TraktID:      traktID,
		ProcessedAt:  time.Now(),
	}

	if err := s.handleNotificationStatus(ctx, processedNotification, media); err != nil {
		return err
	}

	s.cleanupTransferHistory(media, traktID)
	return nil
}

func (s *NotificationService) validateNotification(notification *models.Notification) (int64, *models.Media, error) {
	if notification.Category != categoryMomenarr {
		log.WithField("category", notification.Category).Debug("Ignoring notification for different category")
		return 0, nil, nil
	}

	traktID, err := notification.GetTraktID()
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"name":     notification.Name,
			"category": notification.Category,
			"status":   notification.Status,
			"trakt":    notification.Trakt,
		}).Error("Failed to parse Trakt ID from notification")
		return 0, nil, fmt.Errorf("parsing Trakt ID: %w", err)
	}

	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"trakt":    traktID,
			"name":     notification.Name,
			"category": notification.Category,
		}).Error("Failed to find media record for Trakt ID")
		return 0, nil, fmt.Errorf("finding media for Trakt ID %d: %w", traktID, err)
	}

	log.WithFields(log.Fields{
		"trakt":          traktID,
		"media_title":    media.Title,
		"media_on_disk":  media.OnDisk,
		"download_id":    media.DownloadID,
		"notification":   notification.Name,
		"status":         notification.Status,
	}).Info("Processing notification for media")

	return traktID, media, nil
}

func (s *NotificationService) handleNotificationStatus(ctx context.Context, notification *models.ProcessedNotification, media *models.Media) error {
	if notification.IsSuccess() {
		if err := s.handleDownloadSuccessWithContext(ctx, notification, media); err != nil {
			return fmt.Errorf("handling download success: %w", err)
		}
	} else {
		if err := s.handleDownloadFailureWithContext(ctx, notification); err != nil {
			return fmt.Errorf("handling download failure: %w", err)
		}
	}
	return nil
}

func (s *NotificationService) cleanupTransferHistory(media *models.Media, traktID int64) {
	if err := s.deleteFromHistory(media); err != nil {
		log.WithError(err).WithField("trakt", traktID).Error("Failed to delete from history")
	}

	log.WithFields(log.Fields{
		"trakt":  traktID,
		"title":  media.Title,
		"status": "processed",
	}).Info("Successfully processed notification")
}

// handleDownloadSuccess handles successful download notifications
func (s *NotificationService) handleDownloadSuccess(notification *models.ProcessedNotification, media *models.Media) error {
	return s.handleDownloadSuccessWithContext(context.Background(), notification, media)
}

// handleDownloadSuccessWithContext handles successful download notifications with context support
func (s *NotificationService) handleDownloadSuccessWithContext(ctx context.Context, notification *models.ProcessedNotification, media *models.Media) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.logMediaUpdate(notification.TraktID, media)
	return s.markMediaAvailable(media, notification.TraktID)
}

func (s *NotificationService) logMediaUpdate(traktID int64, media *models.Media) {
	log.WithFields(log.Fields{
		"trakt":       traktID,
		"old_disk":    media.OnDisk,
		"new_disk":    true,
		"transfer_id": media.TransferID,
	}).Info("About to update media record")
}

func (s *NotificationService) markMediaAvailable(media *models.Media, traktID int64) error {
	media.OnDisk = true
	media.File = fmt.Sprintf("Premiumize transfer: %s", media.TransferID)
	media.UpdatedAt = time.Now()

	if err := s.repo.SaveMedia(media); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"trakt": traktID,
		}).Error("Failed to update media record in database")
		return fmt.Errorf("updating media record: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt":       traktID,
		"on_disk":     true,
		"transfer_id": media.TransferID,
		"title":       media.Title,
	}).Info("Successfully processed download success")

	return nil
}

// handleDownloadFailure handles failed download notifications
func (s *NotificationService) handleDownloadFailure(notification *models.ProcessedNotification) error {
	return s.handleDownloadFailureWithContext(context.Background(), notification)
}

// handleDownloadFailureWithContext handles failed download notifications with context support
func (s *NotificationService) handleDownloadFailureWithContext(ctx context.Context, notification *models.ProcessedNotification) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

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

// deleteFromHistory deletes a completed transfer from Premiumize
func (s *NotificationService) deleteFromHistory(media *models.Media) error {
	if media.TransferID == "" {
		return fmt.Errorf("media has no transfer ID")
	}

	if err := s.premiumizeClient.DeleteTransfer(context.Background(), media.TransferID); err != nil {
		if err != premiumize.ErrTransferNotFound {
			return fmt.Errorf("deleting transfer from Premiumize: %w", err)
		}
	}

	log.WithFields(log.Fields{
		"trakt":       media.Trakt,
		"transfer_id": media.TransferID,
	}).Debug("Successfully deleted transfer from Premiumize")

	return nil
}


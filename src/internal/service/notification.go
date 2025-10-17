package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	log "github.com/sirupsen/logrus"
)

const (
	statusSuccess            = "SUCCESS"
	includeHiddenHistory     = false
	parseIntBase             = 10
	parseIntBitSize          = 64
	maxNotificationNameLen   = 500
	maxNotificationCategoryLen = 100
)

// NotificationService processes download completion notifications from NZBGet.
// It handles both successful and failed downloads, updating media status and retrying failures.
type NotificationService struct {
	cfg            *config.Config
	mediaRepo      domain.MediaRepository
	nzbRepo        domain.NZBRepository
	downloadClient domain.DownloadClient
	downloadSvc    *DownloadService
}

// NewNotificationService creates a new NotificationService with the provided dependencies.
func NewNotificationService(cfg *config.Config, mediaRepo domain.MediaRepository, nzbRepo domain.NZBRepository, downloadClient domain.DownloadClient, downloadSvc *DownloadService) *NotificationService {
	return &NotificationService{
		cfg:            cfg,
		mediaRepo:      mediaRepo,
		nzbRepo:        nzbRepo,
		downloadClient: downloadClient,
		downloadSvc:    downloadSvc,
	}
}

func (s *NotificationService) Process(ctx context.Context, notification *domain.Notification) error {
	if err := validateNotification(notification); err != nil {
		return fmt.Errorf("invalid notification: %w", err)
	}

	if notification.Category != s.cfg.NZBCategory {
		return nil
	}

	media, err := s.getMedia(ctx, notification.TraktID)
	if err != nil {
		return fmt.Errorf("getting media: %w", err)
	}

	if notification.Status == statusSuccess {
		if err := s.handleSuccess(ctx, notification, media); err != nil {
			return fmt.Errorf("handling success: %w", err)
		}
		s.logSuccessfulDownload(media)
	} else {
		if err := s.handleFailure(ctx, notification); err != nil {
			return fmt.Errorf("handling failure: %w", err)
		}
	}

	return s.deleteFromHistory(ctx, media)
}

func (s *NotificationService) getMedia(ctx context.Context, traktIDStr string) (*domain.Media, error) {
	traktID, err := strconv.ParseInt(traktIDStr, parseIntBase, parseIntBitSize)
	if err != nil {
		return nil, fmt.Errorf("parsing traktID: %w", err)
	}

	media, err := s.mediaRepo.Get(ctx, traktID)
	if err != nil {
		return nil, fmt.Errorf("finding media: %w", err)
	}
	return media, nil
}

func (s *NotificationService) handleSuccess(ctx context.Context, notification *domain.Notification, media *domain.Media) error {
	return s.updateMediaFile(ctx, media, notification.Dir)
}

func (s *NotificationService) updateMediaFile(ctx context.Context, media *domain.Media, filePath string) error {
	if err := validateFilePath(filePath); err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	media.File = filePath
	media.OnDisk = true

	if err := s.mediaRepo.Update(ctx, media.TraktID, media); err != nil {
		return fmt.Errorf("updating media: %w", err)
	}
	return nil
}

func validateNotification(n *domain.Notification) error {
	if n == nil {
		return fmt.Errorf("notification is nil")
	}
	if n.Name == "" {
		return fmt.Errorf("notification name is empty")
	}
	if n.Category == "" {
		return fmt.Errorf("notification category is empty")
	}
	if n.Status == "" {
		return fmt.Errorf("notification status is empty")
	}
	if n.TraktID == "" {
		return fmt.Errorf("notification traktID is empty")
	}
	if len(n.Name) > maxNotificationNameLen {
		return fmt.Errorf("notification name too long: %d", len(n.Name))
	}
	if len(n.Category) > maxNotificationCategoryLen {
		return fmt.Errorf("notification category too long: %d", len(n.Category))
	}
	return nil
}

func validateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "..") {
		return fmt.Errorf("path traversal detected: %s", path)
	}
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("path must be absolute: %s", path)
	}
	return nil
}

func (s *NotificationService) handleFailure(ctx context.Context, notification *domain.Notification) error {
	if err := s.nzbRepo.MarkFailed(ctx, notification.Name); err != nil {
		return fmt.Errorf("marking nzb as failed: %w", err)
	}

	medias, err := s.mediaRepo.FindNotOnDisk(ctx)
	if err != nil {
		return fmt.Errorf("finding media not on disk: %w", err)
	}

	for _, media := range medias {
		if err := ctx.Err(); err != nil {
			log.WithField("error", err).Debug("context cancelled during retry downloads")
			break
		}
		if err := s.retryDownload(ctx, &media); err != nil {
			log.WithFields(log.Fields{
				"error":   err,
				"traktID": media.TraktID,
			}).Error("failed to retry download")
		}
	}
	return nil
}

func (s *NotificationService) retryDownload(ctx context.Context, media *domain.Media) error {
	nzbs, err := s.nzbRepo.FindByTraktID(ctx, media.TraktID, "", false)
	if err != nil || len(nzbs) == 0 {
		return fmt.Errorf("no nzb found for retry: %w", err)
	}

	return s.downloadSvc.CreateDownload(ctx, media.TraktID, &nzbs[0])
}

func (s *NotificationService) deleteFromHistory(ctx context.Context, media *domain.Media) error {
	for i := 0; i < s.cfg.RetryCount; i++ {
		if deleted, err := s.attemptDelete(ctx, media.DownloadID); deleted {
			return err
		}
		time.Sleep(s.cfg.RetryDelay)
	}
	return nil
}

func (s *NotificationService) attemptDelete(ctx context.Context, downloadID int64) (bool, error) {
	history, err := s.downloadClient.History(ctx, includeHiddenHistory)
	if err != nil {
		return true, fmt.Errorf("getting history: %w", err)
	}

	for _, item := range history {
		if item.NZBID == downloadID {
			return s.deleteItem(ctx, downloadID)
		}
	}
	return false, nil
}

func (s *NotificationService) deleteItem(ctx context.Context, downloadID int64) (bool, error) {
	if err := s.downloadClient.DeleteFromHistory(ctx, downloadID); err != nil {
		return true, fmt.Errorf("deleting from history: %w", err)
	}
	return true, nil
}

func (s *NotificationService) logSuccessfulDownload(media *domain.Media) {
	log.WithFields(log.Fields{
		"traktID": media.TraktID,
		"title":   media.Title,
		"file":    media.File,
	}).Info("download completed and file moved successfully")
}

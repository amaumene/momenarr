package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	log "github.com/sirupsen/logrus"
)

const (
	statusSuccess        = "SUCCESS"
	includeHiddenHistory = false
	parseIntBase         = 10
	parseIntBitSize      = 64
)

type NotificationService struct {
	cfg            *config.Config
	mediaRepo      domain.MediaRepository
	nzbRepo        domain.NZBRepository
	downloadClient domain.DownloadClient
	downloadSvc    *DownloadService
}

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
	biggestFile, err := findBiggestFile(notification.Dir)
	if err != nil {
		return fmt.Errorf("finding biggest file: %w", err)
	}

	destPath, err := s.moveFile(biggestFile)
	if err != nil {
		return fmt.Errorf("moving file: %w", err)
	}

	if err := removeDirectory(notification.Dir); err != nil {
		return fmt.Errorf("removing directory: %w", err)
	}

	return s.updateMediaFile(ctx, media, destPath)
}

func findBiggestFile(dir string) (string, error) {
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
		return "", fmt.Errorf("walking directory: %w", err)
	}
	return biggestFile, nil
}

func (s *NotificationService) moveFile(sourcePath string) (string, error) {
	destPath := filepath.Join(s.cfg.DownloadDir, filepath.Base(sourcePath))
	if err := os.Rename(sourcePath, destPath); err != nil {
		return "", fmt.Errorf("moving file: %w", err)
	}
	return destPath, nil
}

func removeDirectory(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing directory: %w", err)
	}
	return nil
}

func (s *NotificationService) updateMediaFile(ctx context.Context, media *domain.Media, filePath string) error {
	media.File = filePath
	media.OnDisk = true

	if err := s.mediaRepo.Update(ctx, media.TraktID, media); err != nil {
		return fmt.Errorf("updating media: %w", err)
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

package service

import (
	"context"
	"fmt"
	"os"

	"github.com/amaumene/momenarr/internal/domain"
	log "github.com/sirupsen/logrus"
)

var errMediaMissing = fmt.Errorf("media record missing during cleanup")

func completeMediaCleanup(ctx context.Context, mediaRepo domain.MediaRepository, nzbRepo domain.NZBRepository, traktID int64, title string) error {
	media, err := fetchExistingMedia(ctx, mediaRepo, traktID, title)
	if err != nil {
		return err
	}

	if err := deleteMediaRecord(ctx, mediaRepo, traktID, title); err != nil {
		return err
	}

	deleteNZBRecords(ctx, nzbRepo, traktID, title)
	deleteMediaFile(media.File, traktID, title)
	logCleanupSuccess(traktID, title)

	return nil
}

func fetchExistingMedia(ctx context.Context, mediaRepo domain.MediaRepository, traktID int64, title string) (*domain.Media, error) {
	media, err := mediaRepo.Get(ctx, traktID)
	if err != nil {
		log.WithFields(log.Fields{
			"traktID": traktID,
			"title":   title,
			"error":   err,
		}).Warn("media record missing during cleanup")
		return nil, errMediaMissing
	}
	return media, nil
}

func deleteMediaRecord(ctx context.Context, mediaRepo domain.MediaRepository, traktID int64, title string) error {
	if err := mediaRepo.Delete(ctx, traktID); err != nil {
		return fmt.Errorf("deleting media %d %s: %w", traktID, title, err)
	}
	return nil
}

func deleteNZBRecords(ctx context.Context, nzbRepo domain.NZBRepository, traktID int64, title string) {
	if err := nzbRepo.DeleteByTraktID(ctx, traktID); err != nil {
		log.WithFields(log.Fields{
			"traktID": traktID,
			"title":   title,
			"error":   err,
		}).Warn("failed to delete nzb records, continuing")
	}
}

func deleteMediaFile(filePath string, traktID int64, title string) {
	if filePath == "" {
		return
	}

	if err := removeFile(filePath); err != nil {
		log.WithFields(log.Fields{
			"traktID": traktID,
			"title":   title,
			"file":    filePath,
			"error":   err,
		}).Warn("failed to delete file, continuing")
	}
}

func removeFile(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("removing file %s: %w", filePath, err)
	}
	return nil
}

func logCleanupSuccess(traktID int64, title string) {
	log.WithFields(log.Fields{
		"traktID": traktID,
		"title":   title,
	}).Info("media cleaned up successfully")
}

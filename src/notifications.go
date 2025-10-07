package main

import (
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	historyRetryCount       = 3
	historyRetryDelay       = 10 * time.Second
	historyDeleteCommand    = "HistoryFinalDelete"
	categoryMomenarr        = "momenarr"
	statusSuccess           = "SUCCESS"
	parseIntBase            = 10
	parseIntBitSize         = 64
	includeHiddenHistory    = false
)

func downloadSuccess(notification Notification, app App, media Media) error {
	file, err := findBiggestFile(notification.Dir)
	if err != nil {
		return fmt.Errorf("finding biggest file: %w", err)
	}

	destPath, err := moveFileToDownloadDir(file, app.Config.DownloadDir)
	if err != nil {
		return err
	}

	if err := removeDownloadDir(notification.Dir); err != nil {
		return err
	}

	media.File = destPath
	media.OnDisk = true

	if err := updateMediaInStore(app, notification.Trakt, media); err != nil {
		return err
	}
	return nil
}

func moveFileToDownloadDir(file string, downloadDir string) (string, error) {
	destPath := filepath.Join(downloadDir, filepath.Base(file))
	err := os.Rename(file, destPath)
	if err != nil {
		return emptyString, fmt.Errorf("moving file to download directory: %w", err)
	}
	return destPath, nil
}

func removeDownloadDir(dir string) error {
	err := os.RemoveAll(dir)
	if err != nil {
		return fmt.Errorf("removing download directory: %w", err)
	}
	return nil
}

func updateMediaInStore(app App, traktStr string, media Media) error {
	traktID, err := parseTraktID(traktStr)
	if err != nil {
		return err
	}

	if err := app.Store.Update(traktID, &media); err != nil {
		return fmt.Errorf("update media path/status in database: %w", err)
	}
	return nil
}

func parseTraktID(traktStr string) (int64, error) {
	traktID, err := strconv.ParseInt(traktStr, parseIntBase, parseIntBitSize)
	if err != nil {
		return 0, fmt.Errorf("converting notification.Trakt to int64: %w", err)
	}
	return traktID, nil
}

func downloadFailure(notification Notification, app App) error {
	if err := markNZBAsFailed(app, notification.Name); err != nil {
		return err
	}

	if err := app.downloadNotOnDisk(); err != nil {
		return fmt.Errorf("downloading on disk: %w", err)
	}
	return nil
}

func markNZBAsFailed(app App, name string) error {
	err := app.Store.UpdateMatching(&NZB{}, bolthold.Where("Title").Eq(name), func(record interface{}) error {
		update, ok := record.(*NZB)
		if !ok {
			return fmt.Errorf("record isn't the correct type! Wanted NZB, got %T", record)
		}
		update.Failed = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("updating NZB record: %w", err)
	}
	return nil
}

func deleteFromHistory(media Media, app App) error {
	for i := 0; i < historyRetryCount; i++ {
		if deleted, err := attemptHistoryDelete(media, app); deleted {
			return err
		}
		time.Sleep(historyRetryDelay)
	}
	return nil
}

func attemptHistoryDelete(media Media, app App) (bool, error) {
	history, err := app.NZBGet.History(includeHiddenHistory)
	if err != nil {
		return true, fmt.Errorf("getting NZBGet history: %w", err)
	}

	for _, item := range history {
		if item.NZBID == media.DownloadID {
			return deleteHistoryItem(app, media.DownloadID)
		}
	}
	return false, nil
}

func deleteHistoryItem(app App, downloadID int64) (bool, error) {
	IDs := []int64{downloadID}
	result, err := app.NZBGet.EditQueue(historyDeleteCommand, emptyString, IDs)
	if err != nil || !result {
		return true, fmt.Errorf("failed to delete NZBGet download: %w", err)
	}
	return true, nil
}

func processNotification(notification Notification, app App) error {
	if notification.Category != categoryMomenarr {
		return nil
	}

	media, err := getMediaFromNotification(notification, app)
	if err != nil {
		return err
	}

	if err := handleNotificationStatus(notification, app, media); err != nil {
		return err
	}

	if err := deleteFromHistory(media, app); err != nil {
		return fmt.Errorf("deleting from history: %w", err)
	}
	return nil
}

func getMediaFromNotification(notification Notification, app App) (Media, error) {
	var media Media
	traktID, err := parseTraktID(notification.Trakt)
	if err != nil {
		return media, err
	}

	err = app.Store.Get(traktID, &media)
	if err != nil {
		return media, fmt.Errorf("finding media: %w", err)
	}
	return media, nil
}

func handleNotificationStatus(notification Notification, app App, media Media) error {
	if notification.Status == statusSuccess {
		if err := downloadSuccess(notification, app, media); err != nil {
			return fmt.Errorf("downloading success: %w", err)
		}
	} else {
		if err := downloadFailure(notification, app); err != nil {
			return fmt.Errorf("downloading failure: %w", err)
		}
	}
	return nil
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
		return "", err
	}
	return biggestFile, nil
}

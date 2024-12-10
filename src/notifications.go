package main

import (
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"os"
	"path/filepath"
)

func downloadSuccess(notification Success, app App, media Media) error {
	file, err := findBiggestFile(notification.Dir)
	if err != nil {
		return fmt.Errorf("finding biggest file: %v", err)
	}

	destPath := filepath.Join(app.Config.DownloadDir, filepath.Base(file))
	err = os.Rename(file, destPath)
	if err != nil {
		return fmt.Errorf("moving file to download directory: %v", err)
	}

	err = os.RemoveAll(notification.Dir)
	if err != nil {
		return fmt.Errorf("removing download directory: %v", err)
	}

	media.File = destPath
	media.OnDisk = true
	media.DownloadID = "downloaded"
	if err := app.Store.Update(media.IMDB, &media); err != nil {
		return fmt.Errorf("update media path/status in database: %v", err)
	}
	return nil
}

func downloadFailure(notification Failure, app App) error {
	err := app.Store.UpdateMatching(&NZB{}, bolthold.Where("Title").Eq(notification.Message), func(record interface{}) error {
		update, ok := record.(*NZB)
		if !ok {
			return fmt.Errorf("record isn't the correct type! Wanted NZB, got %T", record)
		}
		update.Failed = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("updating NZB record: %v", err)
	}
	var nzbs []NZB
	err = app.Store.Find(&nzbs, bolthold.Where("Title").Eq(notification.Message))
	if err != nil {
		return fmt.Errorf("finding NZB record: %v", err)
	}
	for _, nzb := range nzbs {
		var media Media
		err = app.Store.Get(nzb.IMDB, &media)
		if err != nil {
			return fmt.Errorf("finding media: %s: %v", nzb.IMDB, err)
		}
		media.OnDisk = false
		media.DownloadID = ""
		if err := app.Store.Update(nzb.IMDB, &media); err != nil {
			return fmt.Errorf("update media status in database: %v", err)
		}
	}
	if err = app.downloadNotOnDisk(); err != nil {
		return fmt.Errorf("downloading on disk: %v", err)
	}
	return nil
}

//func deleteFromHistory(media Media, app App) error {
//for i := 0; i < 3; i++ {
//	history, err := app.NZBGet.History(false)
//	if err != nil {
//		return fmt.Errorf("getting NZBGet history: %v", err)
//	}
//	for _, item := range history {
//		if item.NZBID == media.DownloadID {
//			IDs := []int64{
//				media.DownloadID,
//			}
//			result, err := app.NZBGet.EditQueue("HistoryFinalDelete", "", IDs)
//			if err != nil || result == false {
//				return fmt.Errorf("failed to delete NZBGet download: %v", err)
//			} else {
//				return nil
//			}
//		}
//	}
//	time.Sleep(10 * time.Second)
//}
//return nil
//}

func processSuccess(notification Success, app App) error {
	var media []Media
	err := app.Store.Find(&media, bolthold.Where("DownloadID").Eq(notification.Id).Limit(1))
	if err != nil {
		return fmt.Errorf("finding media: %v", err)
	}
	if len(media) > 0 {
		if notification.Status == "" {
			if err = downloadSuccess(notification, app, media[0]); err != nil {
				return fmt.Errorf("downloading success: %v", err)
			}
		}
		//if err = deleteFromHistory(media[0], app); err != nil {
		//	return fmt.Errorf("deleting from history: %v", err)
		//}
	}
	return nil
}

func processFailure(notification Failure, app App) error {
	fmt.Printf("notification: %+v\n", notification)
	if err := downloadFailure(notification, app); err != nil {
		return fmt.Errorf("downloading failure: %v", err)
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

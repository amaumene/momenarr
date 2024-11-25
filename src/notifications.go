package main

import (
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
)

func processNotification(notification Notification, app App) error {
	if notification.Category == "momenarr" {
		var media Media
		err := app.Store.Get(notification.IMDB, &media)
		if err != nil {
			return fmt.Errorf("finding media: %v", err)
		}
		if notification.Status == "SUCCESS" {
			file, err := findBiggestFile(notification.Dir)
			if err != nil {
				log.WithFields(log.Fields{"err": err}).Error("Finding biggest file")
			}

			destPath := filepath.Join(app.Config.DownloadDir, filepath.Base(file))
			err = os.Rename(file, destPath)
			if err != nil {
				log.WithFields(log.Fields{"err": err}).Error("Moving file to download directory")
			}
			media.File = destPath
			media.OnDisk = true
			if err := app.Store.Update(notification.IMDB, &media); err != nil {
				log.WithFields(log.Fields{"err": err}).Error("Update media path/status in database")
			}
		} else {
			err := app.Store.UpdateMatching(&NZB{}, bolthold.Where("Title").Eq(notification.Name), func(record interface{}) error {
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
			if err = app.downloadNotOnDisk(); err != nil {
				return fmt.Errorf("downloading on disk: %v", err)
			}
		}
		for i := 0; i < 3; i++ {
			history, err := app.NZBGet.History(false)
			if err != nil {
				log.WithFields(log.Fields{"err": err}).Error("Getting NZBGet history")
			}
			for _, item := range history {
				if item.NZBID == media.DownloadID {
					IDs := []int64{
						media.DownloadID,
					}
					result, err := app.NZBGet.EditQueue("HistoryFinalDelete", "", IDs)
					if err != nil || result == false {
						log.WithFields(log.Fields{"err": err, "ID": media.DownloadID}).Error("Failed to delete NZBGet download")
					}
				}
			}
			time.Sleep(10 * time.Second)
		}
	}
	return nil
}

// findBiggestFile finds the biggest file in the given directory and its subdirectories.
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

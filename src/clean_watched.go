package main

import (
	"fmt"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
)

func (appConfig App) cleanWatched() {
	err := appConfig.processHistoryShows()
	if err != nil {
		log.Fatalf("Error cleaning watched: %v", err)
	}
}

func (appConfig App) processHistoryShows() error {
	params := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	historyParams := &trakt.ListHistoryParams{
		ListParams: params,
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -5),
	}
	iterator := sync.History(historyParams)
	for iterator.Next() {
		item, err := iterator.History()
		if err != nil {
			return fmt.Errorf("error scanning item: %v", err)
		}

		switch item.Type.String() {
		case "movie":
			err = appConfig.removeFile(string(item.Movie.IMDB))
			if err != nil {
				return err
			}
		case "episode":
			err = appConfig.removeFile(string(item.Show.IMDB))
			if err != nil {
				return err
			}
		default:
			log.WithFields(log.Fields{
				"type": item.Type,
			}).Info("Skipping unknown media type")
		}
	}

	if err := iterator.Err(); err != nil {
		return fmt.Errorf("error iterating history: %v", err)
	}

	return nil
}

func (appConfig App) removeFile(IMDB string) error {
	var media Media
	err := appConfig.store.Get(IMDB, &media)
	if err != nil && err.Error() == "No data found for this key" {
		log.WithFields(log.Fields{
			"err":  err,
			"IMDB": IMDB,
		}).Info("No media found")
		return nil
	} else if err != nil {
		return fmt.Errorf("error finding media: %v", err)
	}

	if len(media.File) > 0 {
		err = appConfig.store.Delete(IMDB, &media)
		if err != nil {
			log.WithFields(log.Fields{
				"file": media.File,
			}).Fatal("Deleting media in database")
			return err
		}
		err := os.Remove(filepath.Join(appConfig.downloadDir, media.File))
		if err != nil {
			log.WithFields(log.Fields{
				"file": media.File,
			}).Error("Deleting file")
			return err
		} else {
			log.WithFields(log.Fields{
				"file": media.File,
			}).Info("Deleting file")
		}
	}

	return nil
}

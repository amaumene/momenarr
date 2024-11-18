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
	appConfig.processHistoryShows()
}

func (appConfig App) processHistoryShows() {
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
			log.Fatalf("Error scanning item: %v", err)
		}
		if item.Type.String() == "movie" {
			appConfig.removeFile(string(item.Movie.IMDB))
		}
		if item.Type.String() == "episode" {
			fmt.Println(item.Show.IMDB)
			fmt.Println(item.Show.MediaIDs)
			appConfig.removeFile(string(item.Show.IMDB))
		}
	}

	if err := iterator.Err(); err != nil {
		log.Fatalf("Error iterating history: %v", err)
	}
}

func (appConfig App) removeFile(IMDB string) {
	var media Media
	err := appConfig.store.Get(IMDB, &media)
	if err != nil {
		log.Fatalf("Error finding media: %v", err)
	}
	if len(media.File) > 0 {
		err = appConfig.store.Delete(IMDB, &media)
		if err != nil {
			log.WithFields(log.Fields{
				"file": media.File,
			}).Fatal("Deleting media in database")
		}
		err := os.Remove(filepath.Join(appConfig.downloadDir, media.File))
		if err != nil {
			log.WithFields(log.Fields{
				"file": media.File,
			}).Error("Deleting file")
		} else {
			log.WithFields(log.Fields{
				"file": media.File,
			}).Info("Deleting file")
		}
	}
}

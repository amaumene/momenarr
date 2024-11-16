package main

import (
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
			IMDB, _ := strconv.ParseInt(strings.TrimPrefix(string(item.Movie.IMDB), "tt"), 10, 64)
			appConfig.removeFile(IMDB)
		}
		if item.Type.String() == "episode" {
			IMDB, _ := strconv.ParseInt(strings.TrimPrefix(string(item.Show.IMDB), "tt"), 10, 64)
			appConfig.removeFile(IMDB)
		}
	}

	if err := iterator.Err(); err != nil {
		log.Fatalf("Error iterating history: %v", err)
	}
}

func (appConfig App) removeFile(IMDB int64) {
	var medias []Media
	err := appConfig.store.Find(&medias, bolthold.Where("IMDB").Eq(IMDB).Index("IMDB"))
	if err != nil {
		log.Fatalf("Error finding media: %v", err)
	}
	for _, media := range medias {
		if media.File != "" {
			err = appConfig.store.DeleteMatching(&media, bolthold.Where("IMDB").Eq(IMDB).Index("IMDB"))
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
}

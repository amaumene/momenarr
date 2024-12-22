package main

import (
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

func (app App) cleanWatched() error {
	params := trakt.ListParams{OAuth: app.TraktToken.AccessToken}

	historyParams := &trakt.ListHistoryParams{
		ListParams: params,
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -5),
	}
	iterator := sync.History(historyParams)
	for iterator.Next() {
		item, err := iterator.History()
		if err != nil {
			return fmt.Errorf("scanning watch history: %v", err)
		}

		switch item.Type.String() {
		case "movie":
			err = app.removeMedia(string(item.Movie.IMDB))
			if err != nil {
				return fmt.Errorf("removing movie: %v", err)
			}
		case "episode":
			err = app.removeMedia(string(item.Episode.IMDB))
			if err != nil {
				return fmt.Errorf("removing episode: %v", err)
			}
		}
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating watch history: %v", err)
	}
	return nil
}

func (app App) removeMedia(IMDB string) error {
	var media Media
	err := app.Store.Get(IMDB, &media)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("finding media")
	}
	err = app.Store.DeleteMatching(&NZB{}, bolthold.Where("IMDB").Eq(media.IMDB))
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("deleting NZBs")
	}
	err = os.Remove(media.File)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("deleting media")
	}
	err = app.Store.Delete(IMDB, &media)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("deleting database entry")
	}
	return nil
}

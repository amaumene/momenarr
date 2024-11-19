package main

import (
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
)

func (appConfig *App) syncMoviesDbFromTrakt() error {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "movie",
	}
	iterator := sync.WatchList(watchListParams)

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{
				"item": item,
				"err":  err,
			}).Error("Scanning movie watchlist")
			continue
		}

		movie := Media{
			IMDB:       string(item.Movie.IMDB),
			Title:      item.Movie.Title,
			Year:       item.Movie.Year,
			OnDisk:     false,
			File:       "",
			DownloadID: 0,
		}
		err = appConfig.store.Insert(string(item.Movie.IMDB), movie)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Inserting movie into database")
			return err
		}
	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Iterating movie history")
		return err
	}

	return nil
}

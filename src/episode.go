package main

import (
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/episode"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
)

func (appConfig *App) syncEpisodesDbFromTrakt(show *trakt.Show, ep *trakt.Episode) {
	insert := Media{
		TVDB:   int64(show.TVDB),
		Number: ep.Number,
		Season: ep.Season,
		IMDB:   string(ep.IMDB),
	}
	err := appConfig.store.Insert(ep.IMDB, insert)
	if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Inserting movie into database")
	}
}

func (appConfig *App) getNewEpisodes() {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "show",
	}
	iterator := sync.WatchList(watchListParams)

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{
				"item": item,
				"err":  err,
			}).Fatal("Error scanning item")
		}

		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: appConfig.traktToken.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.WithFields(log.Fields{
				"show": item.Show.Title,
				"err":  err,
			}).Fatal("Error getting show progress")
		}
		appConfig.syncEpisodesDbFromTrakt(item.Show, showProgress.NextEpisode)
		i := 1
		for i < 3 {
			nextEpisode, _ := episode.Get(item.Show.IMDB, showProgress.NextEpisode.Season, showProgress.NextEpisode.Number+int64(i), nil)
			appConfig.syncEpisodesDbFromTrakt(item.Show, nextEpisode)
			i++
		}
	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating watchlist")
	}
}

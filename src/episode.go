package main

import (
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/episode"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
)

func (appConfig *App) syncEpisodeToDB(show *trakt.Show, ep *trakt.Episode) error {
	media := Media{
		TVDB:   int64(show.TVDB),
		Number: ep.Number,
		Season: ep.Season,
		IMDB:   string(ep.IMDB),
	}
	err := appConfig.store.Insert(ep.IMDB, media)
	if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
		log.WithFields(log.Fields{"err": err}).Error("Inserting episode into database")
		return err
	}
	return nil
}

func (appConfig *App) getNewEpisodes() error {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}
	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "show",
	}
	iterator := sync.WatchList(watchListParams)

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{"item": item, "err": err}).Error("Error scanning item")
			return err
		}

		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: appConfig.traktToken.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.WithFields(log.Fields{"show": item.Show.Title, "err": err}).Error("Error getting show progress")
			return err
		}

		if err := appConfig.syncEpisodeToDB(item.Show, showProgress.NextEpisode); err != nil {
			return err
		}

		for i := 1; i < 3; i++ {
			nextEpisode, err := episode.Get(item.Show.IMDB, showProgress.NextEpisode.Season, showProgress.NextEpisode.Number+int64(i), nil)
			if err != nil {
				log.WithFields(log.Fields{"show": item.Show.Title, "err": err}).Error("Error getting next episode")
				return err
			}
			if err := appConfig.syncEpisodeToDB(item.Show, nextEpisode); err != nil {
				return err
			}
		}
	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error iterating watchlist")
		return err
	}
	return nil
}

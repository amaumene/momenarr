package main

import (
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/episode"
	"github.com/amaumene/momenarr/trakt/show"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

func (app App) insertEpisodeToDB(show *trakt.Show, ep *trakt.Episode) error {
	if len(show.IMDB) > 0 || int64(show.TVDB) > 0 || ep.Number > 0 || ep.Season > 0 {
		media := Media{
			TVDB:   int64(show.TVDB),
			Number: ep.Number,
			Season: ep.Season,
			IMDB:   string(ep.IMDB),
			Title:  ep.Title,
			Year:   show.Year,
		}
		err := app.Store.Insert(ep.IMDB, media)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			return fmt.Errorf("inserting episode into database: %v", err)
		}
	}
	return nil
}

func (app App) syncEpisodesFromFavorites() (error, []interface{}) {
	tokenParams := trakt.ListParams{OAuth: app.TraktToken.AccessToken}
	params := &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       "shows",
	}
	iterator := sync.Favorites(params)

	var episodes []interface{}
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("scanning episode item")
		}
		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: app.TraktToken.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("getting show progress")
		}
		if showProgress.NextEpisode != nil {
			for i := 0; i < 3; i++ {
				nextEpisode, err := episode.Get(item.Show.IMDB, showProgress.NextEpisode.Season, showProgress.NextEpisode.Number+int64(i), nil)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("getting next episode from database")
				} else if err := app.insertEpisodeToDB(item.Show, nextEpisode); err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("inserting episode into database")
				}
				episodes = append(episodes, string(nextEpisode.IMDB))
			}
		}
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating episode watchlist: %v", err), nil
	}
	return nil, episodes
}

func (app App) syncEpisodesFromWatchlist() (error, []interface{}) {
	tokenParams := trakt.ListParams{OAuth: app.TraktToken.AccessToken}
	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "show",
	}
	iterator := sync.WatchList(watchListParams)

	var episodes []interface{}
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("scanning episode item")
		}
		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: app.TraktToken.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("getting show progress")
		} else if err := app.insertEpisodeToDB(item.Show, showProgress.NextEpisode); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("inserting episode into database")
		}
		episodes = append(episodes, string(item.Show.IMDB))
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating episode watchlist: %v", err), nil
	}
	return nil, episodes
}

func (app App) syncEpisodesFromTrakt() (error, []interface{}) {
	err, watchlist := app.syncEpisodesFromWatchlist()
	if err != nil {
		return err, nil
	}
	err, favorites := app.syncEpisodesFromFavorites()
	if err != nil {
		return err, nil
	}
	mergedEpisodes := append(watchlist, favorites...)
	return nil, mergedEpisodes
}

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
	if len(show.IMDB) == 0 || int64(show.TVDB) == 0 || ep.Number < 0 || ep.Season < 0 {
		log.WithFields(log.Fields{
			"media": show.Title,
		}).Error("episode missing IMDB, TVDB, episode number or season number")
	} else {
		media := Media{
			TVDB:   int64(show.TVDB),
			Number: ep.Number,
			Season: ep.Season,
			IMDB:   string(ep.IMDB),
			Title:  ep.Title,
			Year:   show.Year,
		}
		err := app.Store.Upsert(ep.IMDB, media)
		if err != nil {
			return fmt.Errorf("upserting episode into database: %v", err)
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
			return fmt.Errorf("scanning episode item: %v", err), nil
		}
		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: app.TraktToken.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			return fmt.Errorf("getting show progress: %v", err), nil
		}
		if showProgress.NextEpisode != nil {
			for i := 0; i < 3; i++ {
				nextEpisode, err := episode.Get(item.Show.IMDB, showProgress.NextEpisode.Season, showProgress.NextEpisode.Number+int64(i), nil)
				if err != nil {
					return fmt.Errorf("getting next episode from database: %v", err), nil
				}
				if err := app.insertEpisodeToDB(item.Show, nextEpisode); err != nil {
					return err, nil
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
			return fmt.Errorf("scanning episode item: %v", err), nil
		}
		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: app.TraktToken.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			return fmt.Errorf("getting show progress: %v", err), nil
		}
		if err := app.insertEpisodeToDB(item.Show, showProgress.NextEpisode); err != nil {
			return err, nil
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

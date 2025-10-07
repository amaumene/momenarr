package main

import (
	"errors"
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/episode"
	"github.com/amaumene/momenarr/trakt/show"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

const (
	typeShows              = "shows"
	typeShow               = "show"
	nextEpisodesCount      = 3
	errDuplicateKey        = "This Key already exists in this bolthold for this type"
)

var (
	ErrNoEpisodesFound = errors.New("no episodes found")
)

func (app App) insertEpisodeToDB(show *trakt.Show, ep *trakt.Episode) error {
	if !app.isValidEpisode(show, ep) {
		return nil
	}

	media := app.buildMediaFromEpisode(show, ep)
	err := app.Store.Insert(int64(ep.Trakt), media)
	if err != nil && err.Error() != errDuplicateKey {
		return fmt.Errorf("inserting episode into database: %w", err)
	}
	return nil
}

func (app App) isValidEpisode(show *trakt.Show, ep *trakt.Episode) bool {
	return int64(ep.Trakt) > 0 && len(show.IMDB) > 0 && ep.Number > 0 && ep.Season > 0
}

func (app App) buildMediaFromEpisode(show *trakt.Show, ep *trakt.Episode) Media {
	return Media{
		Trakt:  int64(ep.Trakt),
		Number: ep.Number,
		Season: ep.Season,
		IMDB:   string(show.IMDB),
		Title:  ep.Title,
		Year:   show.Year,
	}
}

func (app App) syncEpisodesFromFavorites() ([]interface{}, error) {
	params := app.buildFavoritesParams()
	iterator := sync.Favorites(params)

	episodes, err := app.collectEpisodesFromIterator(iterator)
	if err != nil {
		return nil, fmt.Errorf("iterating episode favorites: %w", err)
	}
	return episodes, nil
}

func (app App) buildFavoritesParams() *trakt.ListFavoritesParams {
	tokenParams := trakt.ListParams{OAuth: app.TraktToken.AccessToken}
	return &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       typeShows,
	}
}

func (app App) collectEpisodesFromIterator(iterator *trakt.FavoritesEntryIterator) ([]interface{}, error) {
	var episodes []interface{}
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("scanning episode item")
			continue
		}

		showEpisodes := app.processShowForEpisodes(item.Show)
		episodes = append(episodes, showEpisodes...)
	}

	if err := iterator.Err(); err != nil {
		return nil, err
	}
	return episodes, nil
}

func (app App) processShowForEpisodes(s *trakt.Show) []interface{} {
	progressParams := &trakt.ProgressParams{
		Params: trakt.Params{OAuth: app.TraktToken.AccessToken},
	}
	showProgress, err := show.WatchedProgress(s.Trakt, progressParams)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("getting show progress")
		return nil
	}

	if showProgress.NextEpisode == nil {
		return nil
	}

	return app.fetchNextEpisodes(s, showProgress.NextEpisode)
}

func (app App) fetchNextEpisodes(show *trakt.Show, nextEp *trakt.Episode) []interface{} {
	var episodes []interface{}
	for i := 0; i < nextEpisodesCount; i++ {
		ep := app.fetchEpisodeWithFallback(trakt.ID(show.Trakt), nextEp.Season, nextEp.Number+int64(i))
		if ep != nil {
			app.storeEpisode(show, ep)
			episodes = append(episodes, int64(ep.Trakt))
		}
	}
	return episodes
}

func (app App) fetchEpisodeWithFallback(showID trakt.SearchID, season, number int64) *trakt.Episode {
	ep, err := episode.Get(showID, season, number, nil)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("getting next episode from trakt")
		ep, err = episode.Get(showID, season+1, 1, nil)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("probably no more episodes")
			return nil
		}
	}
	return ep
}

func (app App) storeEpisode(show *trakt.Show, ep *trakt.Episode) {
	if err := app.insertEpisodeToDB(show, ep); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("inserting episode into database")
	}
}

func (app App) syncEpisodesFromWatchlist() ([]interface{}, error) {
	params := app.buildWatchlistParams()
	iterator := sync.WatchList(params)

	episodes, err := app.collectWatchlistEpisodes(iterator)
	if err != nil {
		return nil, fmt.Errorf("iterating episode watchlist: %w", err)
	}
	return episodes, nil
}

func (app App) buildWatchlistParams() *trakt.ListWatchListParams {
	tokenParams := trakt.ListParams{OAuth: app.TraktToken.AccessToken}
	return &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       typeShow,
	}
}

func (app App) collectWatchlistEpisodes(iterator *trakt.WatchListEntryIterator) ([]interface{}, error) {
	var episodes []interface{}
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("scanning episode item")
			continue
		}

		nextEpisode := app.getNextEpisodeFromShow(item.Show)
		if nextEpisode != nil {
			app.storeEpisode(item.Show, nextEpisode)
			episodes = append(episodes, int64(nextEpisode.Trakt))
		}
	}

	if err := iterator.Err(); err != nil {
		return nil, err
	}
	return episodes, nil
}

func (app App) getNextEpisodeFromShow(s *trakt.Show) *trakt.Episode {
	progressParams := &trakt.ProgressParams{
		Params: trakt.Params{OAuth: app.TraktToken.AccessToken},
	}
	showProgress, err := show.WatchedProgress(s.Trakt, progressParams)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("getting show progress")
		return nil
	}
	return showProgress.NextEpisode
}

func (app App) syncEpisodesFromTrakt() ([]interface{}, error) {
	watchlist, err := app.syncEpisodesFromWatchlist()
	if err != nil {
		return nil, err
	}

	favorites, err := app.syncEpisodesFromFavorites()
	if err != nil {
		return nil, err
	}

	mergedEpisodes := append(watchlist, favorites...)
	if len(mergedEpisodes) == 0 {
		return nil, ErrNoEpisodesFound
	}
	return mergedEpisodes, nil
}

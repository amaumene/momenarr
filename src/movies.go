package main

import (
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

func (app App) insertMovieToDB(movie *trakt.Movie) error {
	if movie.IMDB == "" {
		log.WithFields(log.Fields{
			"media": movie.Title,
		}).Error("movie missing IMDB")
	} else {
		media := Media{
			IMDB:   string(movie.IMDB),
			Title:  movie.Title,
			Year:   movie.Year,
			OnDisk: false,
		}
		err := app.Store.Insert(string(movie.IMDB), media)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			return fmt.Errorf("scanning movie item: %v", err)
		}
	}
	return nil
}

func (app App) syncMoviesFromWatchlist() (error, []interface{}) {
	tokenParams := trakt.ListParams{OAuth: app.TraktToken.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "movie",
	}
	iterator := sync.WatchList(watchListParams)

	var movies []interface{}
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("scanning movie item")
		}
		if err := app.insertMovieToDB(item.Movie); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("inserting movie into database")
		}
		movies = append(movies, string(item.Movie.IMDB))
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating movie watchlist: %v", err), nil
	}
	return nil, movies
}

func (app App) syncMoviesFromFavorites() (error, []interface{}) {
	tokenParams := trakt.ListParams{OAuth: app.TraktToken.AccessToken}
	params := &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       "movies",
	}
	iterator := sync.Favorites(params)

	var movies []interface{}
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("scanning movie item")
		}
		if err := app.insertMovieToDB(item.Movie); err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("inserting movie into database")
		}
		movies = append(movies, string(item.Movie.IMDB))
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating movie favorites: %v", err), nil
	}
	return nil, movies
}

func (app App) syncMoviesFromTrakt() (error, []interface{}) {
	err, watchlist := app.syncMoviesFromWatchlist()
	if err != nil {
		return err, nil
	}
	err, favorites := app.syncMoviesFromFavorites()
	if err != nil {
		return err, nil
	}
	mergedMovies := append(watchlist, favorites...)
	return nil, mergedMovies
}

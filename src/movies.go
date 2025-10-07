package main

import (
	"errors"
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

const (
	typeMovies = "movies"
	typeMovie  = "movie"
)

var (
	ErrNoMoviesFound = errors.New("no movies found")
)

func (app App) insertMovieToDB(movie *trakt.Movie) error {
	if !app.isValidMovie(movie) {
		return nil
	}

	media := app.buildMediaFromMovie(movie)
	err := app.Store.Insert(int64(movie.Trakt), media)
	if err != nil && err.Error() != errDuplicateKey {
		return fmt.Errorf("scanning movie item: %w", err)
	}
	return nil
}

func (app App) isValidMovie(movie *trakt.Movie) bool {
	return int64(movie.Trakt) > 0 && len(movie.IMDB) > 0
}

func (app App) buildMediaFromMovie(movie *trakt.Movie) Media {
	return Media{
		Trakt:  int64(movie.Trakt),
		IMDB:   string(movie.IMDB),
		Title:  movie.Title,
		Year:   movie.Year,
		OnDisk: false,
	}
}

func (app App) syncMoviesFromWatchlist() ([]interface{}, error) {
	params := app.buildMovieWatchlistParams()
	iterator := sync.WatchList(params)

	movies, err := app.collectMoviesFromWatchlist(iterator)
	if err != nil {
		return nil, fmt.Errorf("iterating movie watchlist: %w", err)
	}
	return movies, nil
}

func (app App) buildMovieWatchlistParams() *trakt.ListWatchListParams {
	tokenParams := trakt.ListParams{OAuth: app.TraktToken.AccessToken}
	return &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       typeMovie,
	}
}

func (app App) collectMoviesFromWatchlist(iterator *trakt.WatchListEntryIterator) ([]interface{}, error) {
	var movies []interface{}
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("scanning movie item")
			continue
		}

		app.storeMovie(item.Movie)
		movies = append(movies, int64(item.Movie.Trakt))
	}

	if err := iterator.Err(); err != nil {
		return nil, err
	}
	return movies, nil
}

func (app App) storeMovie(movie *trakt.Movie) {
	if err := app.insertMovieToDB(movie); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("inserting movie into database")
	}
}

func (app App) syncMoviesFromFavorites() ([]interface{}, error) {
	params := app.buildMovieFavoritesParams()
	iterator := sync.Favorites(params)

	movies, err := app.collectMoviesFromFavorites(iterator)
	if err != nil {
		return nil, fmt.Errorf("iterating movie favorites: %w", err)
	}
	return movies, nil
}

func (app App) buildMovieFavoritesParams() *trakt.ListFavoritesParams {
	tokenParams := trakt.ListParams{OAuth: app.TraktToken.AccessToken}
	return &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       typeMovies,
	}
}

func (app App) collectMoviesFromFavorites(iterator *trakt.FavoritesEntryIterator) ([]interface{}, error) {
	var movies []interface{}
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("scanning movie item")
			continue
		}

		app.storeMovie(item.Movie)
		movies = append(movies, int64(item.Movie.Trakt))
	}

	if err := iterator.Err(); err != nil {
		return nil, err
	}
	return movies, nil
}

func (app App) syncMoviesFromTrakt() ([]interface{}, error) {
	watchlist, err := app.syncMoviesFromWatchlist()
	if err != nil {
		return nil, err
	}

	favorites, err := app.syncMoviesFromFavorites()
	if err != nil {
		return nil, err
	}

	mergedMovies := append(watchlist, favorites...)
	if len(mergedMovies) == 0 {
		return nil, ErrNoMoviesFound
	}
	return mergedMovies, nil
}

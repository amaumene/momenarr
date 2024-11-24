package main

import (
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
)

func (appConfig *App) syncMoviesFromWatchlist() error {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "movie",
	}
	iterator := sync.WatchList(watchListParams)

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			return fmt.Errorf("scanning movice item: %v", err)
		}

		movie := Media{
			IMDB:   string(item.Movie.IMDB),
			Title:  item.Movie.Title,
			Year:   item.Movie.Year,
			OnDisk: false,
		}
		err = appConfig.store.Insert(string(item.Movie.IMDB), movie)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			return fmt.Errorf("scanning movie item: %v", err)
		}
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating movie watchlist: %v", err)
	}
	return nil
}

func (appConfig *App) syncMoviesFromFavorites() error {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}
	params := &trakt.ListFavoritesParams{
		ListParams: tokenParams,
		Type:       "movies",
	}
	iterator := sync.Favorites(params)
	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			return fmt.Errorf("scanning movice item: %v", err)
		}

		movie := Media{
			IMDB:   string(item.Movie.IMDB),
			Title:  item.Movie.Title,
			Year:   item.Movie.Year,
			OnDisk: false,
		}
		err = appConfig.store.Insert(string(item.Movie.IMDB), movie)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			return fmt.Errorf("scanning movie item: %v", err)
		}
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating episode watchlist: %v", err)
	}
	return nil
}

func (appConfig *App) syncMoviesFromTrakt() error {
	err := appConfig.syncMoviesFromWatchlist()
	if err != nil {
		return err
	}
	err = appConfig.syncMoviesFromFavorites()
	if err != nil {
		return err
	}
	return nil
}

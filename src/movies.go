package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/newsnab"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"strconv"
	"strings"
)

func (appConfig *App) syncMoviesDbFromTrakt() {
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
			}).Error("Scanning movie history")
		}
		IMDB, _ := strconv.ParseInt(strings.TrimPrefix(string(item.Movie.IMDB), "tt"), 10, 64)
		movie := Movie{
			IMDB:       IMDB,
			Title:      item.Movie.Title,
			Year:       item.Movie.Year,
			OnDisk:     false,
			File:       "",
			DownloadID: 0,
		}
		err = appConfig.store.Insert(IMDB, movie)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Inserting movie into database")
		}
	}
	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Iterating movie history")
	}
}

func (appConfig *App) populateNzbForMovies() {
	movies := []Movie{}
	_ = appConfig.store.Find(&movies, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	for _, movie := range movies {
		jsonResponse, err := newsnab.SearchMovie(movie.IMDB, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{
				"movie": movie.Title,
			}).Error("Searching NZB for movie")
		}

		var feed newsnab.Feed
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Unmarshalling JSON NZB movie")
		}
		if len(feed.Channel.Item) > 0 {
			for _, item := range feed.Channel.Item {
				length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Converting NZB movie Length to int64")
				}
				nzb := NZB{
					ID:     movie.IMDB,
					Link:   item.Link,
					Length: length,
					Title:  item.Title,
				}
				err = appConfig.store.Insert(strings.TrimPrefix(item.GUID, "https://nzbs.in/details/"), nzb)
				if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Inserting NZB movie into database")
				}
			}
		}
	}
}

func (appConfig *App) downloadMovieNotOnDisk() {
	var movies []Movie
	_ = appConfig.store.Find(&movies, bolthold.Where("OnDisk").Eq(false))
	for _, movie := range movies {
		nzb, err := appConfig.getNzbFromDB(movie.IMDB)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		} else {
			appConfig.createOrDownloadCachedMovie(movie.IMDB, nzb)
		}
	}
}

func (appConfig *App) createOrDownloadCachedMovie(IMDB int64, nzb NZB) error {
	torboxDownload, err := appConfig.torBoxClient.CreateUsenetDownload(nzb.Link, nzb.Title)
	if err != nil {
		log.WithFields(log.Fields{
			"title":  nzb.Title,
			"detail": torboxDownload.Detail,
			"err":    err,
		}).Error("Creating TorBox transfer")
	}
	if torboxDownload.Success {
		err = appConfig.store.UpdateMatching(&Movie{}, bolthold.Where("IMDB").Eq(IMDB).Index("IMDB"), func(record interface{}) error {
			update, ok := record.(*Movie) // record will always be a pointer
			if !ok {
				return fmt.Errorf("Record isn't the correct type!  Wanted Movie, got %T", record)
			}
			update.DownloadID = torboxDownload.Data.UsenetDownloadID
			return nil
		})
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		}
		log.WithFields(log.Fields{
			"IMDB":  IMDB,
			"Title": nzb.Title,
		}).Info("Download started successfully")
	}
	if torboxDownload.Detail == "Found cached usenet download. Using cached download." {
		err = appConfig.downloadCachedData(torboxDownload)
		if err != nil {
			log.WithFields(log.Fields{
				"movie": nzb.Title,
				"err":   err,
			}).Fatal("Error downloading cached data")
		}
	}
	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/newsnab"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"strconv"
	"strings"
)

func (appConfig *App) populateNzbForEpisodes() {
	episodes := []Episode{}
	_ = appConfig.store.Find(&episodes, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	for _, episode := range episodes {
		jsonResponse, err := newsnab.SearchTVShow(episode.TVDB, episode.Season, episode.Number, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{
				"IMDB": episode.IMDB,
			}).Error("Searching NZB for episode")
		}

		var feed newsnab.Feed
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Unmarshalling JSON NZB episode")
		}
		if len(feed.Channel.Item) > 0 {
			for _, item := range feed.Channel.Item {
				length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
				if err != nil {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Converting NZB episode Length to int64")
				}
				nzb := NZB{
					ID:     episode.IMDB,
					Link:   item.Link,
					Length: length,
					Title:  item.Title,
				}
				err = appConfig.store.Insert(strings.TrimPrefix(item.GUID, "https://nzbs.in/details/"), nzb)
				if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
					log.WithFields(log.Fields{
						"err": err,
					}).Error("Inserting NZB episode into database")
				}
			}
		}
	}
}

func (appConfig *App) syncEpisodesDbFromTrakt(show *trakt.Show, episode *trakt.Episode) {
	IMDB, _ := strconv.ParseInt(strings.TrimPrefix(string(episode.IMDB), "tt"), 10, 64)
	insert := Episode{
		TVDB:   int64(show.TVDB),
		Number: episode.Number,
		Season: episode.Season,
		IMDB:   IMDB,
	}
	err := appConfig.store.Insert(IMDB, insert)
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
	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating history")
	}
}

func (appConfig *App) downloadEpisodeNotOnDisk() {
	var episodes []Episode
	_ = appConfig.store.Find(&episodes, bolthold.Where("OnDisk").Eq(false))
	for _, episode := range episodes {
		nzb, err := appConfig.getNzbFromDB(episode.IMDB)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Request NZB from database")
		} else {
			appConfig.createOrDownloadCachedEpisode(episode.IMDB, nzb)
		}
	}
}

func (appConfig *App) createOrDownloadCachedEpisode(IMDB int64, nzb NZB) error {
	torboxDownload, err := appConfig.torBoxClient.CreateUsenetDownload(nzb.Link, nzb.Title)
	if err != nil {
		log.WithFields(log.Fields{
			"title":  nzb.Title,
			"detail": torboxDownload.Detail,
			"err":    err,
		}).Error("Creating TorBox transfer")
	}
	if torboxDownload.Success {
		err = appConfig.store.UpdateMatching(&Episode{}, bolthold.Where("IMDB").Eq(IMDB).Index("IMDB"), func(record interface{}) error {
			update, ok := record.(*Episode) // record will always be a pointer
			if !ok {
				return fmt.Errorf("Record isn't the correct type!  Wanted Episode, got %T", record)
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

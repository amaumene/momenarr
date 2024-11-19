package main

import (
	"encoding/json"
	"github.com/amaumene/momenarr/newsnab"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"strconv"
	"strings"
)

func (appConfig *App) searchNZB(media Media) newsnab.Feed {
	var feed newsnab.Feed
	if media.Number > 0 && media.Season > 0 {
		jsonResponse, err := newsnab.SearchTVShow(media.TVDB, media.Season, media.Number, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{"IMDB": media.IMDB}).Error("Searching NZB for media")
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Unmarshalling JSON NZB media")
		}
	} else {
		jsonResponse, err := newsnab.SearchMovie(media.IMDB, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			log.WithFields(log.Fields{"IMDB": media.IMDB}).Error("Searching NZB for media")
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Unmarshalling JSON NZB media")
		}
	}
	return feed
}

func (appConfig *App) insertNZBItems(media Media, items []newsnab.Item) {
	for _, item := range items {
		length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Converting NZB media Length to int64")
			continue
		}

		nzb := NZB{
			IMDB:   media.IMDB,
			Link:   item.Link,
			Length: length,
			Title:  item.Title,
		}

		err = appConfig.store.Insert(strings.TrimPrefix(item.GUID, "https://nzbs.in/details/"), nzb)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			log.WithFields(log.Fields{"err": err}).Error("Inserting NZB media into database")
		}
	}
}

func (appConfig *App) populateNZB() error {
	var medias []Media
	err := appConfig.store.Find(&medias, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	if err != nil {
		return err
	}

	for _, media := range medias {
		feed := appConfig.searchNZB(media)
		if len(feed.Channel.Item) > 0 {
			appConfig.insertNZBItems(media, feed.Channel.Item)
		}
	}
	return nil
}

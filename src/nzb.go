package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/newsnab"
	"github.com/timshannon/bolthold"
	"strconv"
	"strings"
)

func (appConfig *App) searchNZB(media Media) (newsnab.Feed, error) {
	var feed newsnab.Feed
	if media.Number > 0 && media.Season > 0 {
		jsonResponse, err := newsnab.SearchTVShow(media.TVDB, media.Season, media.Number, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			return feed, fmt.Errorf("searching NZB for episode: %s", err)
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			return feed, fmt.Errorf("unmarshalling JSON NZB episode: %s", err)
		}
	} else {
		jsonResponse, err := newsnab.SearchMovie(media.IMDB, appConfig.newsNabHost, appConfig.newsNabApiKey)
		if err != nil {
			return feed, fmt.Errorf("searching NZB for movie: %s", err)
		}
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			return feed, fmt.Errorf("unmarshalling JSON NZB movie: %s", err)
		}
	}
	return feed, nil
}

func (appConfig *App) insertNZBItems(media Media, items []newsnab.Item) error {
	for _, item := range items {
		length, err := strconv.ParseInt(item.Enclosure.Attributes.Length, 10, 64)
		if err != nil {
			return fmt.Errorf("converting NZB media length to int64: %s", err)
		}

		nzb := NZB{
			IMDB:   media.IMDB,
			Link:   item.Link,
			Length: length,
			Title:  item.Title,
		}

		err = appConfig.store.Insert(strings.TrimPrefix(item.GUID, "https://nzbs.in/details/"), nzb)
		if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
			return fmt.Errorf("inserting NZB media into database: %s", err)
		}
	}
	return nil
}

func (appConfig *App) populateNZB() error {
	var medias []Media
	err := appConfig.store.Find(&medias, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	if err != nil {
		return fmt.Errorf("finding media in database: %s", err)
	}

	for _, media := range medias {
		feed, err := appConfig.searchNZB(media)
		if err != nil {
			return err
		}
		if len(feed.Channel.Item) > 0 {
			err := appConfig.insertNZBItems(media, feed.Channel.Item)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

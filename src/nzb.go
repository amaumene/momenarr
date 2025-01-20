package main

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/newsnab"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func (app App) getNzbFromDB(IMDB string) (NZB, error) {
	var nzb []NZB
	err := app.Store.Find(&nzb, bolthold.Where("IMDB").Eq(IMDB).And("Title").
		RegExp(regexp.MustCompile("(?i)remux")).
		And("Failed").Eq(false).
		SortBy("Length").Reverse().Limit(1).Index("IMDB"))
	if err != nil {
		return NZB{}, fmt.Errorf("request NZB remux from database: %v", err)
	}
	if len(nzb) == 0 {
		err = app.Store.Find(&nzb, bolthold.Where("IMDB").Eq(IMDB).And("Title").
			RegExp(regexp.MustCompile("(?i)web-dl")).
			And("Failed").Eq(false).
			SortBy("Length").Reverse().Limit(1).Index("IMDB"))
		if err != nil {
			return NZB{}, fmt.Errorf("request NZB web-dl from database: %v", err)
		}
	}
	if len(nzb) == 0 {
		err = app.Store.Find(&nzb, bolthold.Where("IMDB").Eq(IMDB).
			And("Failed").Eq(false).
			SortBy("Length").Reverse().Limit(1).Index("IMDB"))
		if err != nil {
			return NZB{}, fmt.Errorf("request NZB no filters from database: %v", err)
		}
	}
	if len(nzb) > 0 {
		return nzb[0], nil
	}
	return NZB{}, fmt.Errorf("no NZB found for %s", IMDB)
}

func (app App) searchNZB(media Media) (newsnab.Feed, error) {
	var feed newsnab.Feed
	if media.Number > 0 && media.Season > 0 {
		xmlResponse, err := newsnab.SearchTVShow(media.IMDBSeason, media.Season, media.Number, app.Config.NewsNabHost, app.Config.NewsNabApiKey)
		if err != nil {
			return feed, fmt.Errorf("searching NZB for episode: %v", err)
		}
		err = xml.Unmarshal([]byte(xmlResponse), &feed)
		if err != nil {
			return feed, fmt.Errorf("unmarshalling XML NZB episode: %v", err)
		}
	} else {
		xmlResponse, err := newsnab.SearchMovie(media.IMDB, app.Config.NewsNabHost, app.Config.NewsNabApiKey)
		if err != nil {
			return feed, fmt.Errorf("searching NZB for movie: %v", err)
		}
		err = xml.Unmarshal([]byte(xmlResponse), &feed)
		if err != nil {
			return feed, fmt.Errorf("unmarshalling XML NZB movie: %v", err)
		}
	}
	return feed, nil
}

func readBlacklist(path string) ([]string, error) {
	var blacklist []string
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return blacklist, fmt.Errorf("opening blacklist file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("error closing file: %v\n", err)
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		blacklist = append(blacklist, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return blacklist, fmt.Errorf("scanning file: %v", err)
	}

	return blacklist, nil
}

func (app App) insertNZBItems(media Media, items []newsnab.Item) error {
	for _, item := range items {
		blacklist, err := readBlacklist(app.Config.DataDir + "/blacklist.txt")
		if err != nil {
			return fmt.Errorf("reading blacklist: %v", err)
		}

		shouldInsert := true
		for _, word := range blacklist {
			if strings.Contains(strings.ToLower(item.Title), strings.ToLower(word)) {
				shouldInsert = false
				break
			}
		}

		if shouldInsert {
			length, err := strconv.ParseInt(item.Enclosure.Length, 10, 64)
			if err != nil {
				return fmt.Errorf("converting NZB media length to int64: %v", err)
			}

			nzb := NZB{
				IMDB:   media.IMDB,
				Link:   item.Enclosure.URL,
				Length: length,
				Title:  item.Title,
			}
			err = app.Store.Insert(strings.TrimPrefix(item.GUID.Value, "https://nzbs.in/details/"), nzb)
			if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
				return fmt.Errorf("inserting NZB media into database: %v", err)
			}
		}
	}
	return nil
}

func (app App) populateNZB() error {
	var medias []Media
	err := app.Store.Find(&medias, bolthold.Where("OnDisk").Eq(false).SortBy("IMDB"))
	if err != nil {
		return fmt.Errorf("finding media in database: %v", err)
	}

	for _, media := range medias {
		feed, err := app.searchNZB(media)
		if err != nil {
			return err
		}
		if len(feed.Channel.Items) > 0 {
			err := app.insertNZBItems(media, feed.Channel.Items)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

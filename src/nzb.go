package main

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/newsnab"
	log "github.com/sirupsen/logrus"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	regexRemux           = "(?i)remux"
	regexWebDL           = "(?i)web-dl"
	nzbQueryLimit        = 1
	nzbMinimumCount      = 0
	nzbFirstIndex        = 0
	blacklistFileName    = "/blacklist.txt"
	blacklistPermissions = 0644
	guidPrefix           = "https://v2.nzbs.in/releases/"
)

var (
	ErrNoNZBFound = errors.New("no NZB found")
)

func (app App) getNzbFromDB(Trakt int64) (NZB, error) {
	nzb, err := app.findNZBByPattern(Trakt, regexRemux)
	if err == nil && len(nzb) > nzbMinimumCount {
		return nzb[nzbFirstIndex], nil
	}

	nzb, err = app.findNZBByPattern(Trakt, regexWebDL)
	if err == nil && len(nzb) > nzbMinimumCount {
		return nzb[nzbFirstIndex], nil
	}

	nzb, err = app.findNZBWithoutPattern(Trakt)
	if err == nil && len(nzb) > nzbMinimumCount {
		return nzb[nzbFirstIndex], nil
	}

	return NZB{}, fmt.Errorf("no NZB found for %d: %w", Trakt, ErrNoNZBFound)
}

func (app App) findNZBByPattern(trakt int64, pattern string) ([]NZB, error) {
	var nzb []NZB
	err := app.Store.Find(&nzb, app.buildNZBQueryWithPattern(trakt, pattern))
	if err != nil {
		return nil, fmt.Errorf("request NZB from database: %w", err)
	}
	return nzb, nil
}

func (app App) findNZBWithoutPattern(trakt int64) ([]NZB, error) {
	var nzb []NZB
	err := app.Store.Find(&nzb, app.buildNZBQueryNoPattern(trakt))
	if err != nil {
		return nil, fmt.Errorf("request NZB from database: %w", err)
	}
	return nzb, nil
}

func (app App) buildNZBQueryWithPattern(trakt int64, pattern string) *bolthold.Query {
	return bolthold.Where("Trakt").Eq(trakt).
		And("Title").RegExp(regexp.MustCompile(pattern)).
		And("Failed").Eq(false).
		SortBy("Length").Reverse().Limit(nzbQueryLimit).Index("Trakt")
}

func (app App) buildNZBQueryNoPattern(trakt int64) *bolthold.Query {
	return bolthold.Where("Trakt").Eq(trakt).
		And("Failed").Eq(false).
		SortBy("Length").Reverse().Limit(nzbQueryLimit).Index("Trakt")
}

func (app App) searchNZB(media Media) (newsnab.Feed, error) {
	if app.isEpisode(media) {
		return app.searchEpisodeNZB(media)
	}
	return app.searchMovieNZB(media)
}

func (app App) isEpisode(media Media) bool {
	return media.Number > nzbMinimumCount && media.Season > nzbMinimumCount
}

func (app App) searchEpisodeNZB(media Media) (newsnab.Feed, error) {
	var feed newsnab.Feed
	xmlResponse, err := newsnab.SearchTVShow(media.IMDB, media.Season, media.Number, app.Config.NewsNabHost, app.Config.NewsNabApiKey)
	if err != nil {
		return feed, fmt.Errorf("searching NZB for episode: %w", err)
	}

	err = xml.Unmarshal([]byte(xmlResponse), &feed)
	if err != nil {
		return feed, fmt.Errorf("unmarshalling XML NZB episode: %w", err)
	}
	return feed, nil
}

func (app App) searchMovieNZB(media Media) (newsnab.Feed, error) {
	var feed newsnab.Feed
	xmlResponse, err := newsnab.SearchMovie(media.IMDB, app.Config.NewsNabHost, app.Config.NewsNabApiKey)
	if err != nil {
		return feed, fmt.Errorf("searching NZB for movie: %w", err)
	}

	err = xml.Unmarshal([]byte(xmlResponse), &feed)
	if err != nil {
		return feed, fmt.Errorf("unmarshalling XML NZB movie: %w", err)
	}
	return feed, nil
}

func readBlacklist(path string) ([]string, error) {
	file, err := openBlacklistFile(path)
	if err != nil {
		return nil, err
	}
	defer closeFile(file)

	return scanBlacklistFile(file)
}

func openBlacklistFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, blacklistPermissions)
	if err != nil {
		return nil, fmt.Errorf("opening blacklist file: %w", err)
	}
	return file, nil
}

func scanBlacklistFile(file *os.File) ([]string, error) {
	var blacklist []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		blacklist = append(blacklist, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning file: %w", err)
	}
	return blacklist, nil
}

func closeFile(file *os.File) {
	if err := file.Close(); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("error closing file")
	}
}

func (app App) insertNZBItems(media Media, items []newsnab.Item) error {
	blacklist := app.loadBlacklist()

	for _, item := range items {
		if err := app.insertNZBItem(media, item, blacklist); err != nil {
			return err
		}
	}
	return nil
}

func (app App) loadBlacklist() []string {
	blacklist, err := readBlacklist(app.Config.DataDir + blacklistFileName)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("blacklist file not found, continuing without filtering")
		return []string{}
	}
	return blacklist
}

func (app App) insertNZBItem(media Media, item newsnab.Item, blacklist []string) error {
	if isBlacklisted(item.Title, blacklist) {
		return nil
	}

	nzb, err := buildNZBFromItem(media, item)
	if err != nil {
		return err
	}

	key := generateNZBKey(item.GUID.Value)
	err = app.Store.Insert(key, nzb)
	if err != nil && err.Error() != errDuplicateKey {
		return fmt.Errorf("inserting NZB media into database: %w", err)
	}
	return nil
}

func isBlacklisted(title string, blacklist []string) bool {
	lowerTitle := strings.ToLower(title)
	for _, word := range blacklist {
		if strings.Contains(lowerTitle, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func buildNZBFromItem(media Media, item newsnab.Item) (NZB, error) {
	length, err := strconv.ParseInt(item.Enclosure.Length, parseIntBase, parseIntBitSize)
	if err != nil {
		return NZB{}, fmt.Errorf("converting NZB media length to int64: %w", err)
	}

	return NZB{
		Trakt:  media.Trakt,
		Link:   item.Enclosure.URL,
		Length: length,
		Title:  item.Title,
	}, nil
}

func generateNZBKey(guidValue string) string {
	return strings.TrimPrefix(guidValue, guidPrefix)
}

func (app App) populateNZB() error {
	medias, err := app.findMediaNotOnDisk()
	if err != nil {
		return err
	}

	for _, media := range medias {
		if err := app.processMediaForNZB(media); err != nil {
			return err
		}
	}
	return nil
}

func (app App) findMediaNotOnDisk() ([]Media, error) {
	var medias []Media
	err := app.Store.Find(&medias, bolthold.Where("OnDisk").Eq(false).SortBy("Trakt"))
	if err != nil {
		return nil, fmt.Errorf("finding media in database: %w", err)
	}
	return medias, nil
}

func (app App) processMediaForNZB(media Media) error {
	feed, err := app.searchNZB(media)
	if err != nil {
		return fmt.Errorf("searching NZB for %v: %w", media, err)
	}

	if len(feed.Channel.Items) > nzbMinimumCount {
		return app.insertNZBItemsForMedia(media, feed)
	}

	log.WithField("media", media).Warn("No NZB found for media")
	return nil
}

func (app App) insertNZBItemsForMedia(media Media, feed newsnab.Feed) error {
	err := app.insertNZBItems(media, feed.Channel.Items)
	if err != nil {
		return fmt.Errorf("inserting NZB items into database: %w", err)
	}
	return nil
}

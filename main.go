package main

import (
	"encoding/xml"
	"fmt"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	"log"
	"os"
	"strconv"
)

func setUpTrakt() *trakt.Token {
	trakt.Key = os.Getenv("TRAKT_API_KEY")
	clientSecret := os.Getenv("TRAKT_CLIENT_SECRET")

	if trakt.Key == "" || clientSecret == "" {
		log.Fatalf("TRAKT_API_KEY and TRAKT_CLIENT_SECRET must be set in environment variables")
	}

	tokenPath := os.Getenv("TOKEN_PATH")
	if tokenPath == "" {
		log.Printf("TOKEN_PATH not set, using current directory")
		tokenPath = "."
	}
	tokenFile := tokenPath + "/token.json"

	token, err := getToken(clientSecret, tokenFile)
	if err != nil {
		log.Fatalf("Error getting token: %v", err)
	}
	return token
}

func getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64) {
	fmt.Printf("Episode: S%dE%d\n", showProgress.NextEpisode.Season, episodeNum)

	xmlResponse, err := searchTVShow(item.Show.TVDB, int(showProgress.NextEpisode.Season), int(episodeNum))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var rss Rss
	// Unmarshal the XML data into the struct
	err = xml.Unmarshal([]byte(xmlResponse), &rss)
	if err != nil {
		log.Fatalf("Error unmarshaling XML: %v", err)
	}

	// Find the item with the biggest length
	var maxItem Item
	var maxLength int

	for _, item := range rss.Channel.Items {
		length, err := strconv.Atoi(item.Enclosure.Length)
		if err != nil {
			log.Printf("Error converting length to integer: %v", err)
			continue
		}
		if length > maxLength {
			maxLength = length
			maxItem = item
		}
	}

	fmt.Printf("Item with the biggest length:\n")
	fmt.Printf("Title: %s\n", maxItem.Title)
	fmt.Printf("Size: %s\n", maxItem.Enclosure.Length)
	fmt.Printf("Link: %s\n", maxItem.Enclosure.URL)
}

var newsNabHost string
var newsNabApiKey string

func main() {
	newsNabApiKey = os.Getenv("NEWSNAB_API_KEY")
	if newsNabApiKey == "" {
		log.Fatalf("NEWSNAB_API_KEY empty. Example: 12345678901234567890123456789012")
	}
	newsNabHost = os.Getenv("NEWSNAB_HOST")
	if newsNabHost == "" {
		log.Fatalf("NEWSNAB_HOST empty. Example: nzbs.com, no need for http:// or https://, it has to be https however")
	}

	token := setUpTrakt()

	tokenParams := trakt.ListParams{OAuth: token.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
	}
	iterator := sync.WatchList(watchListParams)

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.Fatalf("Error scanning item: %v", err)
		}
		fmt.Printf("%s\n", item.Show.Title)
		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: token.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.Fatalf("Error getting show progress: %v", err)
		}

		newEpisode := 3
		for i := 0; i < newEpisode; i++ {
			episodeNum := showProgress.NextEpisode.Number + int64(i)
			getNextEpisodes(showProgress, item, episodeNum)
		}
	}

	if err := iterator.Err(); err != nil {
		log.Fatalf("Error iterating history: %v", err)
	}
}

package main

import (
	"encoding/xml"
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type App struct {
	downloadDir        string
	tempDir            string
	newsNabHost        string
	newsNabApiKey      string
	traktToken         *trakt.Token
	TorBoxClient       torbox.TorBox
	TorBoxMoviesFolder string
	TorBoxShowsFolder  string
}

func findBiggest(rss Rss) Item {
	// Find the item with the biggest length
	var maxItem Item
	var maxLength int

	for _, item := range rss.Channel.Items {
		if (strings.Contains(strings.ToLower(item.Title), "bluray") || strings.Contains(strings.ToLower(item.Title), "blu-ray")) && !strings.Contains(strings.ToLower(item.Title), "remux") {
			continue
		}
		if strings.Contains(strings.ToLower(item.Title), "h265") {
			continue
		}
		//if blacklistFile != "" && strings.Contains(strings.ToLower(item.Title), strings.ToLower(blacklistFile)) {
		//	fmt.Printf("Skipping %s\n", item.Title)
		//	continue
		//}

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
	return maxItem
}

func getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64, appConfig App) {
	xmlResponse, err := searchTVShow(item.Show.TVDB, int(showProgress.NextEpisode.Season), int(episodeNum), appConfig)
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

	maxItem := findBiggest(rss)

	exists, err := fileExists(maxItem.Title, appConfig.downloadDir)
	if err != nil {
		log.Fatalf("Error checking file existence: %v", err)
	}

	if exists {
		fmt.Printf("File already exists on disk, skipping download\n")
		return
	}
	//uploadFileWithRetries(maxItem.Enclosure.URL, maxItem.Title)
}

func getNewEpisodes(appConfig App) {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "show",
	}
	iterator := sync.WatchList(watchListParams)

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.Fatalf("Error scanning item: %v", err)
		}

		progressParams := &trakt.ProgressParams{
			Params: trakt.Params{OAuth: appConfig.traktToken.AccessToken},
		}
		showProgress, err := show.WatchedProgress(item.Show.Trakt, progressParams)
		if err != nil {
			log.Fatalf("Error getting show progress: %v", err)
		}

		newEpisode := 3
		for i := 0; i < newEpisode; i++ {
			episodeNum := showProgress.NextEpisode.Number + int64(i)
			getNextEpisodes(showProgress, item, episodeNum, appConfig)
		}

	}

	if err := iterator.Err(); err != nil {
		log.Fatalf("Error iterating history: %v", err)
	}
}

func getNewMovies(appConfig App) {
	tokenParams := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	watchListParams := &trakt.ListWatchListParams{
		ListParams: tokenParams,
		Type:       "movie",
	}
	iterator := sync.WatchList(watchListParams)

	for iterator.Next() {
		item, err := iterator.Entry()
		if err != nil {
			log.Fatalf("Error scanning item: %v", err)
		}
		xmlResponse, err := searchMovie(item.Movie.IMDB, appConfig)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		var rss Rss
		err = xml.Unmarshal([]byte(xmlResponse), &rss)
		if err != nil {
			log.Fatalf("Error unmarshaling XML: %v", err)
		}
		maxItem := findBiggest(rss)

		fmt.Printf("Choosen file: %s, date: %s\n", maxItem.Title, maxItem.PubDate)

		UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(maxItem.Enclosure.URL, maxItem.Title)
		if err != nil {
			log.Fatalf("Error creating transfer: %v", err)
		}
		if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
			fmt.Printf("Usenet download ID: %d", UsenetCreateDownloadResponse.Data.UsenetDownloadID)
			UsenetDownload, err := appConfig.TorBoxClient.FindDownloadByID(UsenetCreateDownloadResponse.Data.UsenetDownloadID)
			if err != nil {
				log.Fatalf("Error finding download: %v", err)
			}
			err = downloadFromTorBox(UsenetDownload, appConfig)
			if err != nil {
				log.Fatalf("Error downloading file: %v", err)
			}
		}
	}

	if err := iterator.Err(); err != nil {
		log.Fatalf("Error iterating history: %v", err)
	}
}

func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = setUpTrakt(appConfig, traktApiKey, traktClientSecret)
	appConfig.TorBoxClient = torbox.NewTorBoxClient(getEnvTorBox())

	getNewMovies(appConfig)

	http.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		handlePostData(w, r, appConfig)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := "0.0.0.0:3000"
	fmt.Printf("Server is running on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

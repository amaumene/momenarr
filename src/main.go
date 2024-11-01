package main

import (
	"encoding/xml"
	"fmt"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	apiURL            = "https://api.torbox.app/v1/api/usenet/mylist"
	requestDLURL      = "https://api.torbox.app/v1/api/usenet/requestdl"
	createUsenetDLURL = "https://api.torbox.app/v1/api/usenet/createusenetdownload"
	controlUsenetURL  = "https://api.torbox.app/v1/api/usenet/controlusenetdownload"
	maxRetries        = 3
	retryDelay        = 2 * time.Second
)

var (
	torboxApiKey      string
	downloadDir       string
	tempDir           string
	httpClient        = &http.Client{}
	newsNabHost       string
	newsNabApiKey     string
	traktApiKey       string
	traktClientSecret string
)

func findBiggest(rss Rss) Item {
	// Find the item with the biggest length
	var maxItem Item
	var maxLength int

	for _, item := range rss.Channel.Items {
		if (strings.Contains(strings.ToLower(item.Title), "bluray") || strings.Contains(strings.ToLower(item.Title), "blu-ray")) && !strings.Contains(strings.ToLower(item.Title), "remux") {
			continue
		}

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

	fmt.Printf("Choosen file: %s\n", maxItem.Title)
	return maxItem
}

func getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64) {
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

	maxItem := findBiggest(rss)

	exists, err := fileExists(maxItem.Title)
	if err != nil {
		log.Fatalf("Error checking file existence: %v", err)
	}

	if exists {
		fmt.Printf("File already exists on disk, skipping download\n")
		return
	}
	uploadFileWithRetries(maxItem.Enclosure.URL, maxItem.Title)
}

func getNewEpisodes(token *trakt.Token) {
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
		if item.Show == nil {
			xmlResponse, err := searchMovie(item.Movie.IMDB)
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
			uploadFileWithRetries(maxItem.Enclosure.URL, maxItem.Title)
		}
		if item.Movie == nil {
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
	}

	if err := iterator.Err(); err != nil {
		log.Fatalf("Error iterating history: %v", err)
	}
}

func init() {
	traktApiKey = os.Getenv("TRAKT_API_KEY")
	traktClientSecret = os.Getenv("TRAKT_CLIENT_SECRET")

	if traktApiKey == "" || traktClientSecret == "" {
		log.Fatalf("TRAKT_API_KEY and TRAKT_CLIENT_SECRET must be set in environment variables")
	}
	newsNabApiKey = os.Getenv("NEWSNAB_API_KEY")
	if newsNabApiKey == "" {
		log.Fatalf("NEWSNAB_API_KEY empty. Example: 12345678901234567890123456789012")
	}
	newsNabHost = os.Getenv("NEWSNAB_HOST")
	if newsNabHost == "" {
		log.Fatalf("NEWSNAB_HOST empty. Example: nzbs.com, no need for https://")
	}
	torboxApiKey = os.Getenv("TORBOX_API_KEY")
	if torboxApiKey == "" {
		log.Fatal("TORBOX_API_KEY must be set in environment variables")
	}
	downloadDir = os.Getenv("DOWNLOAD_DIR")
	if downloadDir == "" {
		log.Fatal("DOWNLOAD_DIR must be set in environment variables")
	}
	// Create if it doesn't exist
	createDir(downloadDir)

	tempDir = os.Getenv("TEMP_DIR")
	if tempDir == "" {
		log.Fatal("TEMP_DIR environment variable is not set")
	}
	// Create if it doesn't exist
	createDir(tempDir)

	// Clean
	cleanDir(tempDir)
}

func main() {
	token := setUpTrakt()

	go func() {
		for {
			getNewEpisodes(token)
			time.Sleep(1 * time.Hour)
		}
	}()

	http.HandleFunc("/api/data", handlePostData)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	port := ":3000"
	fmt.Printf("Server is running on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

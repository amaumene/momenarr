package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/amaumene/momenarr/internal/torbox"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/show"
	"github.com/jacklaaa89/trakt/sync"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"sort"
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
	db                 *sql.DB
}

type Downloads struct {
	ID   string
	Item []Item `xml:"item"`
}

var (
	currentDownloads []Downloads
)

func sortNZBsMovies(Feed Feed, year int) []Item {
	// Filter the slice before sorting
	filteredItems := []Item{}
	for _, item := range Feed.Channel.Item {
		titleLower := strings.ToLower(item.Title)
		movieYearStr := fmt.Sprintf(".%d.", year)
		if strings.Contains(titleLower, "remux") || strings.Contains(titleLower, "2160p") || strings.Contains(titleLower, "1080p") {
			if strings.Contains(item.Title, movieYearStr) {
				filteredItems = append(filteredItems, item)
			}
		}
	}
	Feed.Channel.Item = filteredItems

	sort.Slice(Feed.Channel.Item, func(i, j int) bool {
		// Sort by 'remux' in title first
		iIsRemux := strings.Contains(strings.ToLower(Feed.Channel.Item[i].Title), "remux")
		jIsRemux := strings.Contains(strings.ToLower(Feed.Channel.Item[j].Title), "remux")

		if iIsRemux && !jIsRemux {
			return true
		}
		if !iIsRemux && jIsRemux {
			return false
		}

		// Then sort by length
		return Feed.Channel.Item[i].Enclosure.Attributes.Length > Feed.Channel.Item[j].Enclosure.Attributes.Length
	})
	var nzbs []Item
	nzbs = append(nzbs, Feed.Channel.Item...)
	return nzbs
}

func sortNZBsShows(Feed Feed, show *trakt.Show) Feed {
	sort.Slice(Feed.Channel.Item, func(i, j int) bool {
		return Feed.Channel.Item[i].Enclosure.Attributes.Length > Feed.Channel.Item[j].Enclosure.Attributes.Length
	})
	sort.Slice(Feed.Channel.Item, func(i, j int) bool {
		// Sort by 'remux' in title first
		iIsRemux := strings.Contains(strings.ToLower(Feed.Channel.Item[i].Title), "remux")
		jIsRemux := strings.Contains(strings.ToLower(Feed.Channel.Item[j].Title), "remux")

		if iIsRemux && !jIsRemux {
			return true
		}
		if !iIsRemux && jIsRemux {
			return false
		}

		// Then sort by length
		return Feed.Channel.Item[i].Enclosure.Attributes.Length > Feed.Channel.Item[j].Enclosure.Attributes.Length
	})
	return Feed
}

func getNextEpisodes(showProgress *trakt.WatchedProgress, item *trakt.WatchListEntry, episodeNum int64, appConfig App) {
	fileName := fmt.Sprintf("%s S%02dE%02d", item.Show.Title, showProgress.NextEpisode.Season, episodeNum)
	if fileExists(fileName, appConfig.downloadDir) == "" {
		xmlResponse, err := searchTVShow(item.Show.TVDB, int(showProgress.NextEpisode.Season), int(episodeNum), appConfig)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		var Feed Feed
		err = xml.Unmarshal([]byte(xmlResponse), &Feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Fatal("Error unmarshalling XML")
		}
		filteredFeed := sortNZBsShows(Feed, item.Show)

		show := findOrCreateData(item.Show.Title + " S" + strconv.Itoa(int(showProgress.NextEpisode.Season)) + "E" + strconv.Itoa(int(episodeNum)))
		show.Item = append(show.Item, filteredFeed.Channel.Item...)
		log.WithFields(log.Fields{
			"name":    show.Item[0].Title,
			"season":  showProgress.NextEpisode.Season,
			"episode": episodeNum,
		}).Info("Going to download")
		UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(show.Item[0].Enclosure.Attributes.URL, show.Item[0].Title)
		if err != nil {
			log.WithFields(log.Fields{
				"show": show.Item[0].Title,
				"err":  err,
			}).Fatal("Error creating transfer")
		}
		if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
			err = downloadCachedData(UsenetCreateDownloadResponse, appConfig)
			if err != nil {
				log.WithFields(log.Fields{
					"show": show.Item[0].Title,
					"err":  err,
				}).Fatal("Error downloading cached data")
			}
		}
	}
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

		newEpisode := 3
		for i := 0; i < newEpisode; i++ {
			episodeNum := showProgress.NextEpisode.Number + int64(i)
			getNextEpisodes(showProgress, item, episodeNum, appConfig)
		}

	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating history")
	}
}

func findOrCreateData(ID string) *Downloads {
	for i := range currentDownloads {
		if currentDownloads[i].ID == ID {
			return &currentDownloads[i]
		}
	}
	// If not found, create a new one and append to currentDownloads
	newMovie := Downloads{ID: ID, Item: []Item{}}
	currentDownloads = append(currentDownloads, newMovie)
	return &currentDownloads[len(currentDownloads)-1]
}

func syncMoviesDb(appConfig App) {
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
			}).Fatal("Error scanning item")
		}
		_, err = appConfig.db.Exec("INSERT INTO movies (imdb_id, title, year) SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM movies WHERE imdb_id = ?)", strings.TrimPrefix(string(item.Movie.IMDB), "tt"), item.Movie.Title, item.Movie.Year, strings.TrimPrefix(string(item.Movie.IMDB), "tt"))
		if err != nil {
			log.WithFields(log.Fields{
				"imdb_id": item.Movie.IMDB,
				"title":   item.Movie.Title,
				"year":    item.Movie.Year,
			}).Fatal("Error inserting movie into database")
		}
	}

	if err := iterator.Err(); err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Fatal("Error iterating history")
	}
}

func downloadCachedData(UsenetCreateDownloadResponse torbox.UsenetCreateDownloadResponse, appConfig App) error {
	log.WithFields(log.Fields{
		"id": UsenetCreateDownloadResponse.Data.UsenetDownloadID,
	}).Info("Found cached usenet download")
	UsenetDownload, err := appConfig.TorBoxClient.FindDownloadByID(UsenetCreateDownloadResponse.Data.UsenetDownloadID)
	if err != nil {
		return err
	}
	if UsenetDownload[0].Cached {
		log.WithFields(log.Fields{
			"name": UsenetDownload[0].Name,
		}).Info("Starting download from cached data")
		err = downloadFromTorBox(UsenetDownload, appConfig)
		if err != nil {
			return err
		}
		err = appConfig.TorBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete")
		if err != nil {
			return err
		}
		return nil
	}
	log.WithFields(log.Fields{
		"name": UsenetDownload[0].Name,
	}).Info("Not really in cache, skipping and hoping to get a notification")
	return nil
}

// Movie represents the structure for the queried movie data.
type Movie struct {
	imdbID     int
	Title      string
	Year       int
	OnDisk     bool
	File       string
	downloadID int
	NZBs       []Item
}

func getMonitoredMovies(appConfig App) []Movie {
	rows, err := appConfig.db.Query("SELECT imdb_id, year FROM movies WHERE on_disk = 0")
	if err != nil {
		log.Fatal(err)
	}
	var movies []Movie
	for rows.Next() {
		var movie Movie

		if err := rows.Scan(&movie.imdbID, &movie.Year); err != nil {
			log.Fatal(err)
		}
		movies = append(movies, movie)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	rows.Close()
	return movies
}

func populateNzbsMovies(appConfig App) error {
	movies := getMonitoredMovies(appConfig)
	for _, movie := range movies {
		jsonResponse, err := searchMovie(movie.imdbID, appConfig)
		if err != nil {
			log.WithFields(log.Fields{"movie": movie}).Fatal("Error searching movie")
			return err
		}

		var feed Feed
		err = json.Unmarshal([]byte(jsonResponse), &feed)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Fatal("Error unmarshalling JSON")
			return err
		}

		filteredFeed := sortNZBsMovies(feed, movie.Year)
		nzbsData, _ := json.Marshal(filteredFeed)
		if len(filteredFeed) > 0 {
			_, err = appConfig.db.Exec(`UPDATE movies SET nzbs = jsonb_set(?) WHERE imdb_id = ?`, nzbsData, movie.imdbID)
			if err != nil {
				return err
			}
			log.WithFields(log.Fields{
				"imdb_id": movie.imdbID,
				"title":   movie.Title,
				"year":    movie.Year,
			}).Info("Updated nzbs field for movie")
		}
	}
	return nil
}

func createOrDownloadCached(appConfig App, link string, title string) error {
	torboxDownload, err := appConfig.TorBoxClient.CreateUsenetDownload(link, title)
	if err != nil {
		log.WithFields(log.Fields{
			"movie": title,
			"err":   err,
		}).Fatal("Error creating transfer")
	}
	if torboxDownload.Detail == "Found cached usenet download. Using cached download." {
		err = downloadCachedData(torboxDownload, appConfig)
		if err != nil {
			log.WithFields(log.Fields{
				"movie": title,
				"err":   err,
			}).Fatal("Error downloading cached data")
		}
	}
	//store the id in the db
	fmt.Println(torboxDownload.Data.UsenetDownloadID)
	return nil
}

func getNewMovies(appConfig App) error {
	err := populateNzbsMovies(appConfig)
	if err != nil {
		log.Fatal("Populating NZBs movies")
		return err
	}
	rows, err := appConfig.db.Query(
		`SELECT json_extract(nzbs.value, '$.title') AS title,
			json_extract(nzbs.value, '$.link') AS link
		FROM movies,
			json_each(movies.nzbs) AS nzbs
		WHERE json_extract(nzbs.value, '$.failed') = 0
		GROUP BY title;`)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var link string
		var title string
		if err := rows.Scan(&title, &link); err != nil {
			log.Fatal(err)
		}
		createOrDownloadCached(appConfig, link, title)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
	rows.Close()
	return nil
}

func main() {
	appConfig := setConfig()
	traktApiKey, traktClientSecret := getEnvTrakt()
	appConfig.traktToken = setUpTrakt(appConfig, traktApiKey, traktClientSecret)
	appConfig.TorBoxClient = torbox.NewTorBoxClient(getEnvTorBox())
	var err error
	appConfig.db, err = sql.Open("sqlite3", "./data.db")
	if err != nil {
		log.Fatal(err)
	}
	_, err = appConfig.db.Exec(`
		CREATE TABLE IF NOT EXISTS movies (
			imdb_id INTEGER PRIMARY KEY,
			title   TEXT	NOT NULL,
			year	INTEGER NOT NULL,
			on_disk INTEGER NOT NULL DEFAULT 0,
			file TEXT  NOT NULL DEFAULT '',
			download_id INTEGER NOT NULL DEFAULT 0,
			nzbs BLOB
		) STRICT;
	`)
	if err != nil {
		log.Fatal(err)
	}

	defer appConfig.db.Close()

	// Create a channel to listen for interrupt signals (e.g., SIGINT)
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)

	// Run a separate goroutine to handle the shutdown signal
	go func() {
		<-shutdownChan
		log.Info("Received shutdown signal, shutting down gracefully...")

		// Close the database connection
		if err := appConfig.db.Close(); err != nil {
			log.Error("Error closing database: ", err)
		}

		// Any other cleanup tasks go here

		log.Info("Server shut down successfully.")
		os.Exit(0)
	}()

	syncMoviesDb(appConfig)
	getNewMovies(appConfig)
	//go func() {
	//	for {
	//		//cleanWatched(appConfig)
	//		getNewMovies(appConfig)
	//		//getNewEpisodes(appConfig)
	//		time.Sleep(6 * time.Hour)
	//	}
	//}()

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

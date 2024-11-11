package main

import (
	"fmt"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/sync"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
)

func cleanWatched(appConfig App) {
	processHistoryShows(appConfig)
}

func processHistoryShows(appConfig App) {
	params := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	historyParams := &trakt.ListHistoryParams{
		ListParams: params,
		Type:       "show",
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -5),
	}
	iterator := sync.History(historyParams)
	for iterator.Next() {
		item, err := iterator.History()
		if err != nil {
			log.Fatalf("Error scanning item: %v", err)
		}
		processEpisode(appConfig, item)
	}

	if err := iterator.Err(); err != nil {
		log.Fatalf("Error iterating history: %v", err)
	}
}

func processEpisode(appConfig App, item *trakt.History) {
	fileName := fmt.Sprintf("%s - S%02dE%02d", item.Show.Title, item.Episode.Season, item.Episode.Number)
	file := fileExists(fileName, appConfig.downloadDir)
	if file != "" {
		fmt.Printf("Deleting %s\n", file)
		err := os.Remove(filepath.Join(appConfig.downloadDir, file))
		if err != nil {
			log.Errorf("Failed to delete file %s: %v", file, err)
		} else {
			log.Infof("Successfully deleted %s", file)
		}
	}
}

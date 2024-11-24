package main

import (
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	"os"
	"path/filepath"
	"time"
)

func (appConfig App) cleanWatched() error {
	params := trakt.ListParams{OAuth: appConfig.traktToken.AccessToken}

	historyParams := &trakt.ListHistoryParams{
		ListParams: params,
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -5),
	}
	iterator := sync.History(historyParams)
	for iterator.Next() {
		item, err := iterator.History()
		if err != nil {
			return fmt.Errorf("scanning watch history: %v", err)
		}

		switch item.Type.String() {
		case "movie":
			err = appConfig.removeFile(string(item.Movie.IMDB))
			if err != nil {
				return fmt.Errorf("removing movie: %v", err)
			}
		case "episode":
			err = appConfig.removeFile(string(item.Show.IMDB))
			if err != nil {
				return fmt.Errorf("removing episode: %v", err)
			}
		}
	}
	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating watch history: %v", err)
	}
	return nil
}

func (appConfig App) removeFile(IMDB string) error {
	var media Media
	err := appConfig.store.Get(IMDB, &media)
	if err != nil && err.Error() == "No data found for this key" {
		return nil
	} else if err != nil {
		return fmt.Errorf("finding media: %v", err)
	}

	if len(media.File) > 0 {
		err = appConfig.store.Delete(IMDB, &media)
		if err != nil {
			return fmt.Errorf("deleting media: %v", err)
		}
		err := os.Remove(filepath.Join(appConfig.downloadDir, media.File))
		if err != nil {
			return fmt.Errorf("deleting file: %v", err)
		}
	}
	return nil
}

package main

import (
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	"os"
	"time"
)

func (app App) cleanWatched() error {
	params := trakt.ListParams{OAuth: app.TraktToken.AccessToken}

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
			err = app.removeMedia(int64(item.Movie.Trakt), item.Movie.Title)
			if err != nil {
				return fmt.Errorf("removing movie: %v", err)
			}
		case "episode":
			err = app.removeMedia(int64(item.Episode.Trakt), item.Show.Title)
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

func (app App) removeMedia(Trakt int64, Name string) error {
	var media Media
	err := app.Store.Get(Trakt, &media)
	if err != nil {
		return fmt.Errorf("finding %d %s in database: %v", Trakt, Name, err)
	}

	err = app.Store.Delete(Trakt, &media)
	if err != nil {
		return fmt.Errorf("deleting database entry for %d %s: %v", Trakt, Name, err)
	}

	err = app.Store.DeleteMatching(&NZB{}, bolthold.Where("Trakt").Eq(media.Trakt))
	if err != nil {
		return fmt.Errorf("deleting NZBs for %d %s: %v", Trakt, Name, err)
	}

	err = os.Remove(media.File)
	if err != nil {
		return fmt.Errorf("deleting %s %s: %v", media.File, Name, err)
	}

	return nil
}

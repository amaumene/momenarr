package main

import (
	"fmt"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	"os"
	"time"
)

const (
	historyLookbackDays = 5
	mediaTypeMovie      = "movie"
	mediaTypeEpisode    = "episode"
)

func (app App) cleanWatched() error {
	historyParams := app.buildHistoryParams()
	iterator := sync.History(historyParams)

	err := app.processWatchHistory(iterator)
	if err != nil {
		return err
	}

	if err := iterator.Err(); err != nil {
		return fmt.Errorf("iterating watch history: %w", err)
	}
	return nil
}

func (app App) buildHistoryParams() *trakt.ListHistoryParams {
	params := trakt.ListParams{OAuth: app.TraktToken.AccessToken}
	return &trakt.ListHistoryParams{
		ListParams: params,
		EndAt:      time.Now(),
		StartAt:    time.Now().AddDate(0, 0, -historyLookbackDays),
	}
}

func (app App) processWatchHistory(iterator *trakt.HistoryIterator) error {
	for iterator.Next() {
		item, err := iterator.History()
		if err != nil {
			return fmt.Errorf("scanning watch history: %w", err)
		}

		if err := app.handleHistoryItem(item); err != nil {
			return err
		}
	}
	return nil
}

func (app App) handleHistoryItem(item *trakt.History) error {
	switch item.Type.String() {
	case mediaTypeMovie:
		return app.removeMovieFromHistory(item)
	case mediaTypeEpisode:
		return app.removeEpisodeFromHistory(item)
	}
	return nil
}

func (app App) removeMovieFromHistory(item *trakt.History) error {
	err := app.removeMedia(int64(item.Movie.Trakt), item.Movie.Title)
	if err != nil {
		return fmt.Errorf("removing movie: %w", err)
	}
	return nil
}

func (app App) removeEpisodeFromHistory(item *trakt.History) error {
	err := app.removeMedia(int64(item.Episode.Trakt), item.Show.Title)
	if err != nil {
		return fmt.Errorf("removing episode: %w", err)
	}
	return nil
}

func (app App) removeMedia(Trakt int64, Name string) error {
	media, err := app.getMediaByTrakt(Trakt, Name)
	if err != nil {
		return err
	}

	if err := app.deleteMediaFromStore(Trakt, media, Name); err != nil {
		return err
	}

	if err := app.deleteAssociatedNZBs(media.Trakt, Name); err != nil {
		return err
	}

	if err := app.deleteMediaFile(media.File, Name); err != nil {
		return err
	}

	return nil
}

func (app App) getMediaByTrakt(trakt int64, name string) (Media, error) {
	var media Media
	err := app.Store.Get(trakt, &media)
	if err != nil {
		return media, fmt.Errorf("finding %d %s in database: %w", trakt, name, err)
	}
	return media, nil
}

func (app App) deleteMediaFromStore(trakt int64, media Media, name string) error {
	err := app.Store.Delete(trakt, &media)
	if err != nil {
		return fmt.Errorf("deleting database entry for %d %s: %w", trakt, name, err)
	}
	return nil
}

func (app App) deleteAssociatedNZBs(trakt int64, name string) error {
	err := app.Store.DeleteMatching(&NZB{}, bolthold.Where("Trakt").Eq(trakt))
	if err != nil {
		return fmt.Errorf("deleting NZBs for %d %s: %w", trakt, name, err)
	}
	return nil
}

func (app App) deleteMediaFile(filePath string, name string) error {
	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("deleting %s %s: %w", filePath, name, err)
	}
	return nil
}

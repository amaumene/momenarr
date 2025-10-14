package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/sync"
	log "github.com/sirupsen/logrus"
)

const (
	mediaTypeMovie   = "movie"
	mediaTypeEpisode = "episode"
)

type CleanupService struct {
	cfg       *config.Config
	mediaRepo domain.MediaRepository
	nzbRepo   domain.NZBRepository
	token     *trakt.Token
}

func NewCleanupService(cfg *config.Config, mediaRepo domain.MediaRepository, nzbRepo domain.NZBRepository, token *trakt.Token) *CleanupService {
	return &CleanupService{
		cfg:       cfg,
		mediaRepo: mediaRepo,
		nzbRepo:   nzbRepo,
		token:     token,
	}
}

func (s *CleanupService) CleanWatched(ctx context.Context) error {
	params := s.buildHistoryParams()
	iterator := sync.History(params)

	for iterator.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}

		item, err := iterator.History()
		if err != nil {
			return fmt.Errorf("scanning history: %w", err)
		}

		if err := s.handleHistoryItem(ctx, item); err != nil {
			log.WithField("error", err).Error("failed to handle watched history item")
		}
	}

	return iterator.Err()
}

func (s *CleanupService) buildHistoryParams() *trakt.ListHistoryParams {
	now := time.Now()
	lookback := time.Duration(s.cfg.HistoryLookbackDays) * 24 * time.Hour
	limit := int64(50)

	return &trakt.ListHistoryParams{
		ListParams: trakt.ListParams{
			OAuth: s.token.AccessToken,
			Limit: &limit,
		},
		EndAt:   now,
		StartAt: now.Add(-lookback),
	}
}

func (s *CleanupService) handleHistoryItem(ctx context.Context, item *trakt.History) error {
	switch item.Type.String() {
	case mediaTypeMovie:
		return s.removeMedia(ctx, int64(item.Movie.Trakt), item.Movie.Title)
	case mediaTypeEpisode:
		return s.removeMedia(ctx, int64(item.Episode.Trakt), item.Show.Title)
	}
	return nil
}

func (s *CleanupService) removeMedia(ctx context.Context, traktID int64, name string) error {
	media, err := s.mediaRepo.Get(ctx, traktID)
	if err != nil {
		return nil
	}

	if err := s.mediaRepo.Delete(ctx, traktID); err != nil {
		return fmt.Errorf("deleting media %d %s: %w", traktID, name, err)
	}

	if err := s.nzbRepo.DeleteByTraktID(ctx, traktID); err != nil {
		log.WithFields(log.Fields{
			"traktID": traktID,
			"name":    name,
			"error":   err,
		}).Warn("failed to delete nzbs, continuing")
	}

	if err := s.deleteFile(media.File, name); err != nil {
		log.WithFields(log.Fields{
			"traktID": traktID,
			"name":    name,
			"file":    media.File,
			"error":   err,
		}).Warn("failed to delete file, continuing")
	}

	s.logRemoval(traktID, name)
	return nil
}

func (s *CleanupService) deleteFile(filePath, name string) error {
	if filePath == "" {
		return nil
	}

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("removing file %s: %w", filePath, err)
	}
	return nil
}

func (s *CleanupService) logRemoval(traktID int64, name string) {
	log.WithFields(log.Fields{
		"traktID": traktID,
		"name":    name,
	}).Info("watched media removed successfully")
}

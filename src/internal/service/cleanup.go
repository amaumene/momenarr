package service

import (
	"context"
	"fmt"
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
	cfg           *config.Config
	mediaRepo     domain.MediaRepository
	nzbRepo       domain.NZBRepository
	tokenProvider domain.TokenProvider
}

func NewCleanupService(cfg *config.Config, mediaRepo domain.MediaRepository, nzbRepo domain.NZBRepository, tokenProvider domain.TokenProvider) *CleanupService {
	return &CleanupService{
		cfg:           cfg,
		mediaRepo:     mediaRepo,
		nzbRepo:       nzbRepo,
		tokenProvider: tokenProvider,
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
			log.WithField("error", err).Warn("failed to handle watched history item")
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
			OAuth: s.tokenProvider.Token().AccessToken,
			Limit: &limit,
		},
		EndAt:   now,
		StartAt: now.Add(-lookback),
	}
}

func (s *CleanupService) handleHistoryItem(ctx context.Context, item *trakt.History) error {
	switch item.Type.String() {
	case mediaTypeMovie:
		if item.Movie == nil {
			log.Warn("movie history item has nil Movie field")
			return nil
		}
		return s.removeMedia(ctx, int64(item.Movie.Trakt), item.Movie.Title)
	case mediaTypeEpisode:
		if item.Show == nil {
			log.Warn("episode history item has nil Show field")
			return nil
		}
		return s.removeMedia(ctx, int64(item.Show.Trakt), item.Show.Title)
	}
	return nil
}

func (s *CleanupService) removeMedia(ctx context.Context, traktID int64, name string) error {
	return completeMediaCleanup(ctx, s.mediaRepo, s.nzbRepo, traktID, name)
}

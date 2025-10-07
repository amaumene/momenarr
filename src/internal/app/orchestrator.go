package app

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	"github.com/amaumene/momenarr/internal/service"
	log "github.com/sirupsen/logrus"
)

type Orchestrator struct {
	cfg         *config.Config
	mediaRepo   domain.MediaRepository
	mediaSvc    *service.MediaService
	nzbSvc      *service.NZBService
	downloadSvc *service.DownloadService
	cleanupSvc  *service.CleanupService
}

func NewOrchestrator(cfg *config.Config, mediaRepo domain.MediaRepository, mediaSvc *service.MediaService, nzbSvc *service.NZBService, downloadSvc *service.DownloadService, cleanupSvc *service.CleanupService) *Orchestrator {
	return &Orchestrator{
		cfg:         cfg,
		mediaRepo:   mediaRepo,
		mediaSvc:    mediaSvc,
		nzbSvc:      nzbSvc,
		downloadSvc: downloadSvc,
		cleanupSvc:  cleanupSvc,
	}
}

func (o *Orchestrator) RunPeriodically(ctx context.Context) {
	ticker := time.NewTicker(o.cfg.TaskInterval)
	defer ticker.Stop()

	o.runTasks(ctx)

	for {
		select {
		case <-ctx.Done():
			log.WithField("component", "orchestrator").Info("stopping background task scheduler")
			return
		case <-ticker.C:
			o.runTasks(ctx)
		}
	}
}

func (o *Orchestrator) runTasks(ctx context.Context) {
	log.WithField("component", "orchestrator").Info("starting scheduled task cycle")

	if err := o.syncMedia(ctx); err != nil {
		log.WithFields(log.Fields{
			"task":  "sync",
			"error": err,
		}).Error("media sync from trakt failed")
	}

	if err := o.populateNZBs(ctx); err != nil {
		log.WithFields(log.Fields{
			"task":  "nzb_search",
			"error": err,
		}).Error("nzb search and population failed")
	}

	if err := o.downloadMedia(ctx); err != nil {
		log.WithFields(log.Fields{
			"task":  "download",
			"error": err,
		}).Error("media download orchestration failed")
	}

	if err := o.cleanupWatched(ctx); err != nil {
		log.WithFields(log.Fields{
			"task":  "cleanup",
			"error": err,
		}).Error("watched media cleanup failed")
	}

	log.WithField("component", "orchestrator").Info("completed scheduled task cycle")
}

func (o *Orchestrator) syncMedia(ctx context.Context) error {
	_, err := o.mediaSvc.SyncFromTrakt(ctx)
	return err
}

func (o *Orchestrator) populateNZBs(ctx context.Context) error {
	return o.nzbSvc.PopulateNZBs(ctx)
}

func (o *Orchestrator) downloadMedia(ctx context.Context) error {
	medias, err := o.mediaRepo.FindNotOnDisk(ctx)
	if err != nil {
		return err
	}

	processedSeasons := make(map[string]bool)

	for _, media := range medias {
		seasonKey := buildSeasonKey(&media)

		if shouldSkipEpisode(&media, processedSeasons) {
			log.WithFields(log.Fields{
				"traktID":   media.TraktID,
				"title":     media.Title,
				"season":    media.Season,
				"episode":   media.Number,
				"seasonKey": seasonKey,
			}).Info("skipping episode, season pack already queued")
			continue
		}

		nzb, err := o.nzbSvc.GetNZB(ctx, media.TraktID)
		if err != nil {
			log.WithFields(log.Fields{
				"traktID": media.TraktID,
				"title":   media.Title,
				"error":   err,
			}).Error("failed to download media item")
			continue
		}

		if isSeasonPackNZB(nzb.Title) {
			log.WithFields(log.Fields{
				"traktID":   media.TraktID,
				"nzbTitle":  nzb.Title,
				"season":    media.Season,
				"seasonKey": seasonKey,
			}).Info("detected season pack, marking season as processed")
			markSeasonAsProcessed(&media, processedSeasons)
		}

		if err := o.downloadSvc.CreateDownload(ctx, media.TraktID, nzb); err != nil {
			log.WithFields(log.Fields{
				"traktID": media.TraktID,
				"title":   media.Title,
				"error":   err,
			}).Error("failed to download media item")
		}
	}
	return nil
}

func shouldSkipEpisode(media *domain.Media, processedSeasons map[string]bool) bool {
	if !isEpisodeMedia(media) {
		return false
	}
	seasonKey := buildSeasonKey(media)
	return processedSeasons[seasonKey]
}

func isEpisodeMedia(media *domain.Media) bool {
	return media.Season > 0 && media.Number > 0
}

func buildSeasonKey(media *domain.Media) string {
	return fmt.Sprintf("%s_S%02d", media.IMDB, media.Season)
}

func markSeasonAsProcessed(media *domain.Media, processedSeasons map[string]bool) {
	if isEpisodeMedia(media) {
		seasonKey := buildSeasonKey(media)
		processedSeasons[seasonKey] = true
	}
}

func isSeasonPackNZB(title string) bool {
	matched, _ := regexp.MatchString(`(?i)(S\d{2}[^E]|Season\s*\d+)`, title)
	return matched
}

func (o *Orchestrator) cleanupWatched(ctx context.Context) error {
	return o.cleanupSvc.CleanWatched(ctx)
}

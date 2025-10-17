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

var seasonPackTitlePattern = regexp.MustCompile(`(?i)(S\d{2}[^E]|Season\s*\d+)`)

type Orchestrator struct {
	cfg         *config.Config
	mediaRepo   domain.MediaRepository
	mediaSvc    *service.MediaService
	nzbSvc      *service.NZBService
	downloadSvc *service.DownloadService
	cleanupSvc  *service.CleanupService
}

type orchestratorTask struct {
	name string
	run  func(context.Context) error
}

type downloadPlanner struct {
	orchestrator     *Orchestrator
	processedSeasons map[string]struct{}
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

	tasks := []orchestratorTask{
		{name: "sync", run: o.syncMedia},
		{name: "nzb_search", run: o.populateNZBs},
		{name: "download", run: o.downloadMedia},
		{name: "cleanup", run: o.cleanupWatched},
	}

	for _, task := range tasks {
		if err := task.run(ctx); err != nil {
			log.WithFields(log.Fields{
				"task":  task.name,
				"error": err,
			}).Error("scheduled task failed")
		}
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

	planner := newDownloadPlanner(o)
	for i := range medias {
		media := medias[i] // Create local copy to avoid pointer aliasing
		planner.handle(ctx, &media)
	}
	return nil
}

func (o *Orchestrator) cleanupWatched(ctx context.Context) error {
	return o.cleanupSvc.CleanWatched(ctx)
}

func newDownloadPlanner(o *Orchestrator) *downloadPlanner {
	return &downloadPlanner{
		orchestrator:     o,
		processedSeasons: make(map[string]struct{}),
	}
}

func (p *downloadPlanner) handle(ctx context.Context, media *domain.Media) {
	seasonKey := buildSeasonKey(media)
	if p.skipEpisode(media, seasonKey) {
		return
	}
	if p.skipExistingDownload(media) {
		return
	}

	nzb := p.fetchNZB(ctx, media)
	if nzb == nil {
		return
	}

	p.registerSeason(media, nzb, seasonKey)
	p.enqueueDownload(ctx, media, nzb)
}

func (p *downloadPlanner) skipEpisode(media *domain.Media, seasonKey string) bool {
	if !isEpisodeMedia(media) {
		return false
	}
	if _, ok := p.processedSeasons[seasonKey]; !ok {
		return false
	}
	p.logSeasonSkip(media, seasonKey)
	return true
}

func (p *downloadPlanner) skipExistingDownload(media *domain.Media) bool {
	if media.DownloadID <= 0 {
		return false
	}
	p.logExistingDownload(media)
	return true
}

func (p *downloadPlanner) fetchNZB(ctx context.Context, media *domain.Media) *domain.NZB {
	nzb, err := p.orchestrator.nzbSvc.GetNZB(ctx, media.TraktID)
	if err != nil {
		p.logNZBError(media, err)
		return nil
	}
	return nzb
}

func (p *downloadPlanner) registerSeason(media *domain.Media, nzb *domain.NZB, seasonKey string) {
	if !isEpisodeMedia(media) {
		return
	}
	if !seasonPackTitlePattern.MatchString(nzb.Title) {
		return
	}
	p.processedSeasons[seasonKey] = struct{}{}
	p.logSeasonDetected(media, nzb, seasonKey)
}

func (p *downloadPlanner) enqueueDownload(ctx context.Context, media *domain.Media, nzb *domain.NZB) {
	if err := p.orchestrator.downloadSvc.CreateDownload(ctx, media.TraktID, nzb); err != nil {
		p.logDownloadError(media, err)
	}
}

func (p *downloadPlanner) logSeasonSkip(media *domain.Media, seasonKey string) {
	log.WithFields(log.Fields{
		"traktID":   media.TraktID,
		"title":     media.Title,
		"season":    media.Season,
		"episode":   media.Number,
		"seasonKey": seasonKey,
	}).Info("skipping episode, season pack already queued")
}

func (p *downloadPlanner) logExistingDownload(media *domain.Media) {
	log.WithFields(log.Fields{
		"traktID":    media.TraktID,
		"title":      media.Title,
		"downloadID": media.DownloadID,
	}).Info("skipping media, download already created")
}

func (p *downloadPlanner) logNZBError(media *domain.Media, err error) {
	log.WithFields(log.Fields{
		"traktID": media.TraktID,
		"title":   media.Title,
		"error":   err,
	}).Error("failed to download media item")
}

func (p *downloadPlanner) logSeasonDetected(media *domain.Media, nzb *domain.NZB, seasonKey string) {
	log.WithFields(log.Fields{
		"traktID":   media.TraktID,
		"nzbTitle":  nzb.Title,
		"season":    media.Season,
		"seasonKey": seasonKey,
	}).Info("detected season pack, marking season as processed")
}

func (p *downloadPlanner) logDownloadError(media *domain.Media, err error) {
	log.WithFields(log.Fields{
		"traktID": media.TraktID,
		"title":   media.Title,
		"error":   err,
	}).Error("failed to download media item")
}

func isEpisodeMedia(media *domain.Media) bool {
	return media.Season > 0 && media.Number > 0
}

func buildSeasonKey(media *domain.Media) string {
	return fmt.Sprintf("%s_S%02d", media.IMDB, media.Season)
}

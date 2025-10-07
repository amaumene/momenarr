package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/internal/clients"
	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	"github.com/amaumene/momenarr/internal/handler"
	"github.com/amaumene/momenarr/internal/service"
	"github.com/amaumene/momenarr/internal/storage"
	"github.com/amaumene/momenarr/nzbget"
	log "github.com/sirupsen/logrus"
)

const (
	shutdownTimeout = 30 * time.Second
)

type App struct {
	cfg          *config.Config
	server       *http.Server
	store        *bolthold.Store
	traktClient  *clients.TraktClient
	orchestrator *Orchestrator
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	store, err := openStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	traktClient, err := initTraktClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing trakt: %w", err)
	}

	app := &App{
		cfg:         cfg,
		store:       store,
		traktClient: traktClient,
	}

	if err := app.wireServices(); err != nil {
		return nil, fmt.Errorf("wiring services: %w", err)
	}

	return app, nil
}

func openStore(cfg *config.Config) (*bolthold.Store, error) {
	store, err := bolthold.Open(cfg.DBPath(), cfg.DBFilePermissions, nil)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return store, nil
}

func initTraktClient(cfg *config.Config) (*clients.TraktClient, error) {
	client, err := clients.NewTraktClient(cfg.TraktAPIKey, cfg.TraktClientSecret, cfg.TokenPath())
	if err != nil {
		return nil, fmt.Errorf("creating trakt client: %w", err)
	}
	return client, nil
}

func (a *App) wireServices() error {
	mediaRepo := storage.NewMediaRepository(a.store)
	nzbRepo := storage.NewNZBRepository(a.store)

	downloadClient, searcher := a.createClients()
	services := a.createServices(mediaRepo, nzbRepo, downloadClient, searcher)

	a.setupHTTPServer(mediaRepo, nzbRepo, services)
	a.orchestrator = NewOrchestrator(a.cfg, mediaRepo, services.media, services.nzb, services.download, services.cleanup)

	return nil
}

func (a *App) createClients() (domain.DownloadClient, domain.NZBSearcher) {
	nzbgetClient := nzbget.New(&nzbget.Config{
		URL:  a.cfg.NZBGetURL,
		User: a.cfg.NZBGetUser,
		Pass: a.cfg.NZBGetPass,
	})
	downloadClient := clients.NewNZBGetAdapter(nzbgetClient)
	searcher := clients.NewNewsnabAdapter(a.cfg.NewsNabHost, a.cfg.NewsNabAPIKey)
	return downloadClient, searcher
}

type services struct {
	download     *service.DownloadService
	nzb          *service.NZBService
	media        *service.MediaService
	cleanup      *service.CleanupService
	notification *service.NotificationService
}

func (a *App) createServices(mediaRepo domain.MediaRepository, nzbRepo domain.NZBRepository, downloadClient domain.DownloadClient, searcher domain.NZBSearcher) services {
	downloadSvc := service.NewDownloadService(a.cfg, mediaRepo, downloadClient)
	nzbSvc := service.NewNZBService(a.cfg, mediaRepo, nzbRepo, searcher)
	mediaSvc := service.NewMediaService(a.cfg, mediaRepo, a.traktClient.Token())
	cleanupSvc := service.NewCleanupService(a.cfg, mediaRepo, nzbRepo, a.traktClient.Token())
	notificationSvc := service.NewNotificationService(a.cfg, mediaRepo, nzbRepo, downloadClient, downloadSvc)

	return services{
		download:     downloadSvc,
		nzb:          nzbSvc,
		media:        mediaSvc,
		cleanup:      cleanupSvc,
		notification: notificationSvc,
	}
}

func (a *App) setupHTTPServer(mediaRepo domain.MediaRepository, nzbRepo domain.NZBRepository, svc services) {
	httpHandler := handler.NewHTTPHandler(a.cfg, mediaRepo, nzbRepo, svc.notification, svc.media, svc.nzb, svc.download)

	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	a.server = &http.Server{
		Addr:    a.cfg.ServerPort,
		Handler: mux,
	}
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go a.traktClient.RefreshPeriodically(ctx, a.cfg.TaskInterval)
	go a.orchestrator.RunPeriodically(ctx)

	go a.startServer()

	return a.waitForShutdown(ctx, cancel)
}

func (a *App) startServer() {
	log.WithFields(log.Fields{
		"component": "server",
		"address":   a.cfg.ServerPort,
	}).Info("http server listening")

	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.WithFields(log.Fields{
			"component": "server",
			"error":     err,
		}).Fatal("http server failed to start")
	}
}

func (a *App) waitForShutdown(ctx context.Context, cancel context.CancelFunc) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		log.WithField("reason", "context_cancelled").Info("initiating graceful shutdown")
	case sig := <-sigChan:
		log.WithField("signal", sig).Info("received shutdown signal")
	}

	cancel()
	return a.shutdown()
}

func (a *App) shutdown() error {
	log.Info("graceful shutdown started")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		log.WithFields(log.Fields{
			"component": "server",
			"error":     err,
		}).Error("http server shutdown failed")
	}

	if err := a.store.Close(); err != nil {
		log.WithFields(log.Fields{
			"component": "database",
			"error":     err,
		}).Error("database connection close failed")
		return err
	}

	log.Info("graceful shutdown completed")
	return nil
}

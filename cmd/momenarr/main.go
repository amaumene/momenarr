package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/pkg/config"
	"github.com/amaumene/momenarr/pkg/handlers"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/services"
	"github.com/amaumene/momenarr/trakt"
	log "github.com/sirupsen/logrus"
)

type app struct {
	config          *config.Config
	repo            repository.Repository
	appService      *services.AppService
	server          *http.Server
	tokenService    *services.TraktTokenService
	allDebridService services.AllDebridInterface
}

func main() {
	initLogging()
	
	app, store, err := initializeApp()
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize application")
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.WithError(err).Error("Failed to close database")
		}
	}()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	var wg sync.WaitGroup
	
	// Start background tasks
	wg.Add(1)
	go app.runBackgroundTasks(ctx, &wg)
	
	// Start HTTP server
	wg.Add(1)
	go app.runHTTPServer(&wg)
	
	log.Info("Momenarr is ready")
	logConfiguration(app.config)
	
	app.waitForShutdown(ctx, cancel, &wg)
}

func initLogging() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.Info("Starting Momenarr with AllDebrid support")
}

func initializeApp() (*app, *bolthold.Store, error) {
	cfg, err := loadAndValidateConfig()
	if err != nil {
		return nil, nil, err
	}
	
	store, err := openDatabase(cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}
	
	repo := repository.NewBoltRepository(store)
	
	// Initialize services
	services, err := initializeServices(cfg, repo)
	if err != nil {
		store.Close()
		return nil, nil, err
	}
	
	// Create HTTP handler and server
	handler := handlers.CreateHandler(services.appService)
	handler.SetupRoutes()
	
	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	
	return &app{
		config:           cfg,
		repo:             repo,
		appService:       services.appService,
		server:           server,
		tokenService:     services.tokenService,
		allDebridService: services.allDebridService,
	}, store, nil
}

func loadAndValidateConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}
	
	return cfg, nil
}

func openDatabase(dataDir string) (*bolthold.Store, error) {
	dbPath := filepath.Join(dataDir, "data.db")
	store, err := bolthold.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return store, nil
}

type serviceContainer struct {
	appService       *services.AppService
	tokenService     *services.TraktTokenService
	allDebridService services.AllDebridInterface
}

func initializeServices(cfg *config.Config, repo repository.Repository) (*serviceContainer, error) {
	// Initialize Trakt
	trakt.Key = cfg.TraktAPIKey
	tokenService := services.NewTraktTokenService(cfg.DataDir, cfg.TraktClientSecret)
	
	traktToken, err := tokenService.GetToken()
	if err != nil {
		return nil, fmt.Errorf("getting Trakt token: %w", err)
	}
	
	// Initialize optional TMDB service
	var tmdbService *services.TMDBService
	if cfg.TMDBAPIKey != "" {
		tmdbService = services.NewTMDBService(cfg.TMDBAPIKey)
		log.Info("TMDB service initialized")
	}
	
	// Create services with proper dependencies
	traktService := createTraktService(repo, traktToken, tmdbService)
	allDebridService := services.NewAllDebridService(repo, cfg.AllDebridAPIKey)
	torrentService := createTorrentService(repo, cfg, traktService, tmdbService)
	downloadService := services.CreateDownloadService(repo, allDebridService, torrentService)
	
	cleanupService := services.CreateCleanupService(repo, allDebridService, traktToken)
	cleanupService.SetWatchedDays(cfg.WatchedDays)
	
	appService := services.CreateAppService(
		repo,
		traktService,
		torrentService,
		downloadService,
		cleanupService,
	)
	
	return &serviceContainer{
		appService:       appService,
		tokenService:     tokenService,
		allDebridService: allDebridService,
	}, nil
}

func createTraktService(repo repository.Repository, token *trakt.Token, tmdb *services.TMDBService) *services.TraktService {
	if tmdb != nil {
		return services.NewTraktServiceWithTMDB(repo, token, tmdb)
	}
	return services.NewTraktService(repo, token)
}

func createTorrentService(repo repository.Repository, cfg *config.Config, traktSvc *services.TraktService, tmdb *services.TMDBService) *services.TorrentService {
	if tmdb != nil {
		return services.CreateTorrentServiceWithTraktAndTMDB(
			repo,
			cfg.BlacklistFile,
			traktSvc,
			tmdb,
		)
	}
	return services.CreateTorrentServiceWithTrakt(repo, cfg.BlacklistFile, traktSvc)
}

func (a *app) runBackgroundTasks(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	
	syncInterval := parseSyncInterval(a.config.SyncInterval)
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()
	
	// Run initial tasks
	if err := a.executeTasksWithRefresh(ctx); err != nil && ctx.Err() == nil {
		log.WithError(err).Error("Initial task run failed")
	}
	
	// Run periodic tasks
	for {
		select {
		case <-ctx.Done():
			log.Info("Background tasks stopped")
			return
		case <-ticker.C:
			if err := a.executeTasksWithRefresh(ctx); err != nil && ctx.Err() == nil {
				log.WithError(err).Error("Scheduled task run failed")
			}
		}
	}
}

func (a *app) executeTasksWithRefresh(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	
	// Refresh Trakt token
	token, err := a.tokenService.GetToken()
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}
	
	refreshedToken, err := a.tokenService.RefreshToken(token)
	if err != nil {
		log.WithError(err).Warn("Token refresh failed, using existing token")
	} else {
		cleanupService := services.CreateCleanupService(a.repo, a.allDebridService, refreshedToken)
		a.appService.UpdateTraktToken(refreshedToken, cleanupService)
		log.Debug("Token refreshed successfully")
	}
	
	// Execute main tasks
	return a.appService.RunTasks(ctx)
}

func (a *app) runHTTPServer(wg *sync.WaitGroup) {
	defer wg.Done()
	
	log.WithField("address", a.server.Addr).Info("Starting HTTP server")
	
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.WithError(err).Error("HTTP server error")
	}
}

func (a *app) waitForShutdown(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	<-sigChan
	log.Info("Shutdown signal received")
	
	// Stop background tasks
	cancel()
	
	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	// Shutdown HTTP server
	go func() {
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			log.WithError(err).Error("HTTP server shutdown error")
		}
	}()
	
	// Wait for all goroutines
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		log.Info("Graceful shutdown completed")
	case <-shutdownCtx.Done():
		log.Warn("Shutdown timeout exceeded")
	}
	
	// Close application service
	if err := a.appService.Close(); err != nil {
		log.WithError(err).Error("Failed to close app service")
	}
}

func parseSyncInterval(interval string) time.Duration {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		log.WithError(err).Warn("Invalid sync interval, using default")
		return 6 * time.Hour
	}
	return duration
}

func logConfiguration(cfg *config.Config) {
	log.WithFields(log.Fields{
		"data_dir":      cfg.DataDir,
		"sync_interval": cfg.SyncInterval,
		"watched_days":  cfg.WatchedDays,
	}).Info("Configuration loaded")
}
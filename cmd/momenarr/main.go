// Package main provides the entry point for the momenarr application.
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

	"github.com/amaumene/gostremiofr/pkg/alldebrid"
	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/pkg/config"
	"github.com/amaumene/momenarr/pkg/handlers"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/services"
	"github.com/amaumene/momenarr/trakt"
	log "github.com/sirupsen/logrus"
)

const (
	defaultSyncInterval = 6 * time.Hour
	shutdownTimeout     = 30 * time.Second
	dbFileMode          = 0600
	dataDirMode         = 0755
	httpReadTimeout     = 15 * time.Second
	httpWriteTimeout    = 15 * time.Second
	httpIdleTimeout     = 30 * time.Second
)

// app contains all application dependencies and services
type app struct {
	config           *config.Config
	repo             repository.Repository
	appService       *services.AppService
	server           *http.Server
	tokenService     *services.TraktTokenService
	allDebridClient  *alldebrid.Client
	apiKey           string
}

func main() {
	initLogging()

	app, store, err := initializeApp()
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize application")
	}
	defer closeStore(store)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	startBackgroundTasks(ctx, &wg, app)
	startHTTPServer(&wg, app)

	log.Info("Momenarr is ready")
	logConfiguration(app.config)

	app.waitForShutdown(ctx, cancel, &wg)
}

// initLogging configures the application logging
func initLogging() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.Info("Starting Momenarr with AllDebrid support")
}

// initializeApp creates and configures the application instance
func initializeApp() (*app, *bolthold.Store, error) {
	cfg, store, repo, err := setupInfrastructure()
	if err != nil {
		return nil, nil, err
	}

	services, err := initializeServices(cfg, repo)
	if err != nil {
		store.Close()
		return nil, nil, err
	}

	return createApp(cfg, repo, services, store)
}

// setupInfrastructure sets up config, database, and repository
func setupInfrastructure() (*config.Config, *bolthold.Store, repository.Repository, error) {
	cfg, err := loadAndValidateConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	store, err := openDatabase(cfg.DataDir)
	if err != nil {
		return nil, nil, nil, err
	}

	repo := repository.NewBoltRepository(store)
	return cfg, store, repo, nil
}

// createApp creates the application instance with all dependencies
func createApp(cfg *config.Config, repo repository.Repository, services *servicesContainer, store *bolthold.Store) (*app, *bolthold.Store, error) {
	server := createHTTPServer(cfg, services.appService)

	return &app{
		config:           cfg,
		repo:            repo,
		appService:      services.appService,
		server:          server,
		tokenService:    services.tokenService,
		allDebridClient: services.allDebridClient,
		apiKey:          services.apiKey,
	}, store, nil
}

// loadAndValidateConfig loads and validates the application configuration
func loadAndValidateConfig() (*config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if err := os.MkdirAll(cfg.DataDir, dataDirMode); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	return cfg, nil
}

// openDatabase opens the BoltDB database
func openDatabase(dataDir string) (*bolthold.Store, error) {
	dbPath := filepath.Join(dataDir, "data.db")
	store, err := bolthold.Open(dbPath, dbFileMode, nil)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	return store, nil
}

// servicesContainer holds initialized services
type servicesContainer struct {
	appService       *services.AppService
	tokenService     *services.TraktTokenService
	allDebridClient  *alldebrid.Client
	apiKey           string
}

// initializeServices creates and configures all application services
func initializeServices(cfg *config.Config, repo repository.Repository) (*servicesContainer, error) {
	trakt.Key = cfg.TraktAPIKey

	tokenService, traktToken, err := setupTraktAuth(cfg)
	if err != nil {
		return nil, err
	}

	tmdbService := initializeTMDB(cfg)

	coreServices := createCoreServices(cfg, repo, traktToken, tmdbService)

	appService := createAppService(repo, coreServices, traktToken)

	return &servicesContainer{
		appService:      appService,
		tokenService:    tokenService,
		allDebridClient: coreServices.allDebrid,
		apiKey:          coreServices.apiKey,
	}, nil
}

// setupTraktAuth sets up Trakt authentication
func setupTraktAuth(cfg *config.Config) (*services.TraktTokenService, *trakt.Token, error) {
	tokenService := services.NewTraktTokenService(cfg.DataDir, cfg.TraktClientSecret)

	traktToken, err := tokenService.GetToken()
	if err != nil {
		return nil, nil, fmt.Errorf("getting Trakt token: %w", err)
	}

	return tokenService, traktToken, nil
}

// coreServices holds the core service dependencies
type coreServices struct {
	trakt     *services.TraktService
	allDebrid *alldebrid.Client
	apiKey    string
	torrent   *services.TorrentService
	download  *services.DownloadService
	cleanup   *services.CleanupService
}

// createCoreServices creates the core services
func createCoreServices(cfg *config.Config, repo repository.Repository, traktToken *trakt.Token, tmdbService *services.TMDBService) *coreServices {
	traktService := createTraktService(repo, traktToken, tmdbService)
	allDebridClient := alldebrid.NewClient()
	torrentService := createTorrentService(repo, cfg, traktService, tmdbService)
	downloadService := services.CreateDownloadService(repo, allDebridClient, cfg.AllDebridAPIKey, torrentService)

	cleanupService := services.CreateCleanupService(repo, allDebridClient, cfg.AllDebridAPIKey, traktToken)
	cleanupService.SetWatchedDays(cfg.WatchedDays)

	return &coreServices{
		trakt:     traktService,
		allDebrid: allDebridClient,
		apiKey:    cfg.AllDebridAPIKey,
		torrent:   torrentService,
		download:  downloadService,
		cleanup:   cleanupService,
	}
}

// createAppService creates the main application service
func createAppService(repo repository.Repository, core *coreServices, traktToken *trakt.Token) *services.AppService {
	return services.CreateAppService(
		repo,
		core.trakt,
		core.torrent,
		core.download,
		core.cleanup,
	)
}

// createTraktService creates TraktService with optional TMDB integration
func createTraktService(repo repository.Repository, token *trakt.Token, tmdb *services.TMDBService) *services.TraktService {
	if tmdb != nil {
		return services.NewTraktServiceWithTMDB(repo, token, tmdb)
	}
	return services.NewTraktService(repo, token)
}

// createTorrentService creates TorrentService with dependencies
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

// runBackgroundTasks manages periodic background tasks
func (a *app) runBackgroundTasks(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	syncInterval := parseSyncInterval(a.config.SyncInterval)
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	if err := a.executeTasksWithRefresh(ctx); err != nil && ctx.Err() == nil {
		log.WithError(err).Error("Initial task run failed")
	}

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

// executeTasksWithRefresh refreshes token and executes scheduled tasks
func (a *app) executeTasksWithRefresh(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := a.refreshTraktToken(); err != nil {
		log.WithError(err).Warn("Token refresh failed, using existing token")
	}

	return a.appService.RunTasks(ctx)
}

// runHTTPServer starts the HTTP server
func (a *app) runHTTPServer(wg *sync.WaitGroup) {
	defer wg.Done()

	log.WithField("address", a.server.Addr).Info("Starting HTTP server")

	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.WithError(err).Error("HTTP server error")
	}
}

// waitForShutdown handles graceful shutdown
func (a *app) waitForShutdown(ctx context.Context, cancel context.CancelFunc, wg *sync.WaitGroup) {
	waitForSignal()
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	a.performShutdown(shutdownCtx, wg)
}

// waitForSignal waits for shutdown signal
func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Info("Shutdown signal received")
}

// performShutdown performs the actual shutdown sequence
func (a *app) performShutdown(ctx context.Context, wg *sync.WaitGroup) {
	shutdownWg := a.startHTTPShutdown(ctx)

	if a.waitForGoroutines(ctx, wg) {
		log.Info("Graceful shutdown completed")
	} else {
		log.Warn("Shutdown timeout exceeded")
	}

	shutdownWg.Wait()
	a.closeAppService()
}

// startHTTPShutdown starts HTTP server shutdown in parallel
func (a *app) startHTTPShutdown(ctx context.Context) *sync.WaitGroup {
	var shutdownWg sync.WaitGroup
	shutdownWg.Add(1)
	go func() {
		defer shutdownWg.Done()
		a.shutdownHTTPServer(ctx)
	}()
	return &shutdownWg
}

// closeAppService closes the app service with error logging
func (a *app) closeAppService() {
	if err := a.appService.Close(); err != nil {
		log.WithError(err).Error("Failed to close app service")
	}
}

// parseSyncInterval parses sync interval with fallback to default
func parseSyncInterval(interval string) time.Duration {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		log.WithError(err).Warn("Invalid sync interval, using default")
		return defaultSyncInterval
	}
	return duration
}

// logConfiguration logs the current configuration
func logConfiguration(cfg *config.Config) {
	log.WithFields(log.Fields{
		"data_dir":      cfg.DataDir,
		"sync_interval": cfg.SyncInterval,
		"watched_days":  cfg.WatchedDays,
	}).Info("Configuration loaded")
}

// closeStore safely closes the database store
func closeStore(store *bolthold.Store) {
	if err := store.Close(); err != nil {
		log.WithError(err).Error("Failed to close database")
	}
}

// startBackgroundTasks starts background task goroutine
func startBackgroundTasks(ctx context.Context, wg *sync.WaitGroup, app *app) {
	wg.Add(1)
	go app.runBackgroundTasks(ctx, wg)
}

// startHTTPServer starts HTTP server goroutine
func startHTTPServer(wg *sync.WaitGroup, app *app) {
	wg.Add(1)
	go app.runHTTPServer(wg)
}

// createHTTPServer creates configured HTTP server
func createHTTPServer(cfg *config.Config, appService *services.AppService) *http.Server {
	handler := handlers.CreateHandler(appService)
	// Note: SetupRoutes() is not called because we're using the handler's ServeHTTP method directly

	return &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}
}

// initializeTMDB creates TMDB service if API key is configured
func initializeTMDB(cfg *config.Config) *services.TMDBService {
	if cfg.TMDBAPIKey == "" {
		return nil
	}

	tmdbService := services.NewTMDBService(cfg.TMDBAPIKey)
	log.Info("TMDB service initialized")
	return tmdbService
}

// refreshTraktToken refreshes Trakt authentication token
func (a *app) refreshTraktToken() error {
	token, err := a.tokenService.GetToken()
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}

	refreshedToken, err := a.tokenService.RefreshToken(token)
	if err != nil {
		return err
	}

	cleanupService := services.CreateCleanupService(a.repo, a.allDebridClient, a.apiKey, refreshedToken)
	a.appService.UpdateTraktToken(refreshedToken, cleanupService)
	log.Debug("Token refreshed successfully")
	return nil
}

// shutdownHTTPServer gracefully shuts down the HTTP server
func (a *app) shutdownHTTPServer(ctx context.Context) {
	if err := a.server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("HTTP server shutdown error")
	}
}

// waitForGoroutines waits for all goroutines with timeout
func (a *app) waitForGoroutines(ctx context.Context, wg *sync.WaitGroup) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

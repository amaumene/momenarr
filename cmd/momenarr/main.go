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
	"github.com/amaumene/momenarr/pkg/premiumize"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/services"
	"github.com/amaumene/momenarr/trakt"
	log "github.com/sirupsen/logrus"
)

func main() {
	initLogging()
	cfg := loadAndValidateConfig()
	store := openDatabase(cfg.DataDir)
	defer closeDatabase(store)

	app := initializeApplication(cfg, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	startServices(ctx, &wg, app, cfg)
	server := startHTTPServer(&wg, cfg, app.handler)
	waitForShutdown(ctx, cancel, server, app.appService, &wg)
}

type application struct {
	appService        *services.AppService
	tokenService      *services.TraktTokenService
	traktToken        *trakt.Token
	repo              repository.Repository
	premiumizeClient  *premiumize.Client
	premiumizeMonitor *services.PremiumizeMonitorService
	handler           *handlers.Handler
}

func initLogging() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	log.Info("Starting Momenarr application")
}

func loadAndValidateConfig() *config.Config {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}
	if err := cfg.Validate(); err != nil {
		log.WithError(err).Fatal("Invalid configuration")
	}
	return cfg
}

func openDatabase(dataDir string) *bolthold.Store {
	dbPath := filepath.Join(dataDir, "data.db")
	store, err := bolthold.Open(dbPath, 0600, nil)
	if err != nil {
		log.WithError(err).Fatal("Failed to open database")
	}
	return store
}

func closeDatabase(store *bolthold.Store) {
	if err := store.Close(); err != nil {
		log.WithError(err).Error("Failed to close database")
	}
}

func initializeApplication(cfg *config.Config, store *bolthold.Store) *application {
	repo := repository.NewBoltRepository(store)
	premiumizeClient := createPremiumizeClient(cfg)
	traktToken, tokenService := initializeTraktServices(cfg)
	services := createServices(cfg, repo, premiumizeClient)
	appService := createAppService(repo, services, premiumizeClient, traktToken)
	handler := setupHandler(appService)

	return &application{
		appService:        appService,
		tokenService:      tokenService,
		traktToken:        traktToken,
		repo:              repo,
		premiumizeClient:  premiumizeClient,
		premiumizeMonitor: services.premiumizeMonitor,
		handler:           handler,
	}
}

func createPremiumizeClient(cfg *config.Config) *premiumize.Client {
	client, err := premiumize.NewClient(&premiumize.Config{
		APIKey: cfg.PremiumizeAPIKey,
	})
	if err != nil {
		log.WithError(err).Fatal("Failed to create Premiumize client")
	}
	return client
}

func initializeTraktServices(cfg *config.Config) (*trakt.Token, *services.TraktTokenService) {
	trakt.Key = cfg.TraktAPIKey
	tokenService := services.NewTraktTokenService(cfg.DataDir, cfg.TraktClientSecret)
	traktToken, err := tokenService.GetToken()
	if err != nil {
		log.WithError(err).Fatal("Failed to get Trakt token")
	}
	return traktToken, tokenService
}

type appServices struct {
	nzbService          *services.NZBService
	downloadService     *services.DownloadService
	notificationService *services.NotificationService
	premiumizeMonitor   *services.PremiumizeMonitorService
}

func createServices(cfg *config.Config, repo repository.Repository, premiumizeClient *premiumize.Client) *appServices {
	nzbService := services.NewNZBService(
		repo,
		cfg.NewsNabHost,
		cfg.NewsNabAPIKey,
		filepath.Join(cfg.DataDir, cfg.BlacklistFile),
	)
	downloadService := services.NewDownloadService(repo, premiumizeClient, nzbService)
	notificationService := services.NewNotificationService(repo, premiumizeClient, downloadService, cfg.DownloadDir)
	premiumizeMonitor := services.NewPremiumizeMonitorService(repo, premiumizeClient, cfg.DownloadDir)

	return &appServices{
		nzbService:          nzbService,
		downloadService:     downloadService,
		notificationService: notificationService,
		premiumizeMonitor:   premiumizeMonitor,
	}
}

func createAppService(repo repository.Repository, svcs *appServices, premiumizeClient *premiumize.Client, traktToken *trakt.Token) *services.AppService {
	appService := services.NewAppService(
		repo,
		nil,
		svcs.nzbService,
		svcs.downloadService,
		svcs.notificationService,
		nil,
	)

	traktService := services.NewTraktService(repo, traktToken)
	cleanupService := services.NewCleanupService(repo, premiumizeClient, traktToken)
	appService.UpdateTraktServices(traktService, cleanupService)

	return appService
}

func setupHandler(appService *services.AppService) *handlers.Handler {
	handler := handlers.NewHandler(appService)
	handler.SetupRoutes()
	return handler
}

func startServices(ctx context.Context, wg *sync.WaitGroup, app *application, cfg *config.Config) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		startBackgroundTasks(ctx, app.appService, app.tokenService, app.traktToken, app.repo, cfg, app.premiumizeClient)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("Starting Premiumize transfer monitor")
		app.premiumizeMonitor.RunPeriodically(ctx, 30*time.Second)
		log.Info("Premiumize transfer monitor stopped")
	}()
}

func startHTTPServer(wg *sync.WaitGroup, cfg *config.Config, handler *handlers.Handler) *http.Server {
	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.WithField("address", server.Addr).Info("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("HTTP server error")
		}
	}()

	return server
}

// startBackgroundTasks starts the background task loop with context cancellation
func startBackgroundTasks(ctx context.Context, appService *services.AppService, tokenService *services.TraktTokenService, currentToken *trakt.Token, repo repository.Repository, cfg *config.Config, premiumizeClient *premiumize.Client) {
	syncInterval, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		log.WithError(err).Error("Invalid sync interval, using default 6h")
		syncInterval = 6 * time.Hour
	}

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	if err := runTasksWithTokenRefresh(ctx, appService, tokenService, &currentToken, repo, premiumizeClient); err != nil {
		log.WithError(err).Error("Failed to run initial tasks")
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("Background tasks cancelled")
			return
		case <-ticker.C:
			if err := runTasksWithTokenRefresh(ctx, appService, tokenService, &currentToken, repo, premiumizeClient); err != nil {
				log.WithError(err).Error("Failed to run scheduled tasks")
			}
		}
	}
}

// runTasksWithTokenRefresh runs tasks and handles token refresh with context
func runTasksWithTokenRefresh(ctx context.Context, appService *services.AppService, tokenService *services.TraktTokenService, currentToken **trakt.Token, repo repository.Repository, premiumizeClient *premiumize.Client) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	refreshTraktToken(tokenService, currentToken, appService, repo, premiumizeClient)

	if err := appService.RunTasks(ctx); err != nil {
		return fmt.Errorf("running application tasks: %w", err)
	}

	return nil
}

func refreshTraktToken(tokenService *services.TraktTokenService, currentToken **trakt.Token, appService *services.AppService, repo repository.Repository, premiumizeClient *premiumize.Client) {
	refreshedToken, err := tokenService.RefreshToken(*currentToken)
	if err != nil {
		log.WithError(err).Error("Failed to refresh Trakt token, using current token")
		return
	}

	*currentToken = refreshedToken
	traktService := services.NewTraktService(repo, refreshedToken)
	cleanupService := services.NewCleanupService(repo, premiumizeClient, refreshedToken)
	appService.UpdateTraktServices(traktService, cleanupService)
	log.Debug("Updated Trakt services with refreshed token")
}

// waitForShutdown waits for shutdown signals and gracefully shuts down
func waitForShutdown(ctx context.Context, cancel context.CancelFunc, server *http.Server, appService *services.AppService, wg *sync.WaitGroup) {
	sig := waitForSignal()
	log.WithField("signal", sig).Info("Received shutdown signal, initiating graceful shutdown")

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	shutdownHTTPServer(server, shutdownCtx)
	waitForGoroutines(wg, shutdownCtx)
	shutdownAppService(appService)

	log.Info("Graceful shutdown completed")
}

func waitForSignal() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	return <-sigChan
}

func shutdownHTTPServer(server *http.Server, ctx context.Context) {
	if err := server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Failed to shutdown HTTP server gracefully")
	} else {
		log.Info("HTTP server shut down successfully")
	}
}

func waitForGoroutines(wg *sync.WaitGroup, ctx context.Context) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("All goroutines completed successfully")
	case <-ctx.Done():
		log.Warn("Shutdown timeout reached, forcing exit")
	}
}

func shutdownAppService(appService *services.AppService) {
	if err := appService.Close(); err != nil {
		log.WithError(err).Error("Failed to shutdown application service")
	} else {
		log.Info("Application service shut down successfully")
	}
}

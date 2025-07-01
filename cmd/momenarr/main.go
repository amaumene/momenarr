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
	"github.com/amaumene/momenarr/nzbget"
	"github.com/amaumene/momenarr/pkg/config"
	"github.com/amaumene/momenarr/pkg/handlers"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/services"
	"github.com/amaumene/momenarr/trakt"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Setup logging
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	log.Info("Starting Momenarr application")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	if err := cfg.Validate(); err != nil {
		log.WithError(err).Fatal("Invalid configuration")
	}

	// Log test mode status
	if cfg.TestMode {
		log.Warn("🧪 RUNNING IN TEST MODE - No downloads will be created, no data will be stored in database")
	}

	// Initialize database
	dbPath := filepath.Join(cfg.DataDir, "data.db")
	store, err := bolthold.Open(dbPath, 0666, nil)
	if err != nil {
		log.WithError(err).Fatal("Failed to open database")
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.WithError(err).Error("Failed to close database")
		}
	}()

	// Initialize repository
	repo := repository.NewBoltRepository(store)

	// Initialize NZBGet client
	nzbGetClient := nzbget.New(&nzbget.Config{
		URL:  cfg.GetNZBGetURL(),
		User: cfg.NZBGetUsername,
		Pass: cfg.NZBGetPassword,
	})

	// Initialize Trakt token service and get token
	trakt.Key = cfg.TraktAPIKey
	tokenService := services.NewTraktTokenService(cfg.DataDir, cfg.TraktClientSecret)
	traktToken, err := tokenService.GetToken()
	if err != nil {
		log.WithError(err).Fatal("Failed to get Trakt token")
	}

	// Initialize services  
	nzbService := services.NewNZBService(
		repo,
		cfg.NewsNabHost,
		cfg.NewsNabAPIKey,
		filepath.Join(cfg.DataDir, cfg.BlacklistFile),
		cfg.TestMode,
	)
	downloadService := services.NewDownloadService(repo, nzbGetClient, nzbService, cfg.TestMode)
	notificationService := services.NewNotificationService(repo, nzbGetClient, downloadService, cfg.DownloadDir)

	// Initialize main application service (we'll pass TraktService later)
	appService := services.NewAppService(
		repo,
		nil, // Will be set after token refresh handling is setup
		nzbService,
		downloadService,
		notificationService,
		nil, // Will be set after token refresh handling is setup
	)

	// Initialize HTTP handlers
	handler := handlers.NewHandler(appService)
	handler.SetupRoutes()

	// Initialize initial Trakt services
	traktService := services.NewTraktService(repo, traktToken)
	cleanupService := services.NewCleanupService(repo, traktToken)
	appService.UpdateTraktServices(traktService, cleanupService)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait group for tracking goroutines
	var wg sync.WaitGroup

	// Start background tasks with context
	wg.Add(1)
	go func() {
		defer wg.Done()
		startBackgroundTasks(ctx, appService, tokenService, traktToken, repo, cfg)
	}()

	// Start HTTP server
	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.WithField("address", server.Addr).Info("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("HTTP server error")
		}
	}()

	// Wait for shutdown signal
	waitForShutdown(ctx, cancel, server, appService, &wg)
}

// startBackgroundTasks starts the background task loop with context cancellation
func startBackgroundTasks(ctx context.Context, appService *services.AppService, tokenService *services.TraktTokenService, currentToken *trakt.Token, repo repository.Repository, cfg *config.Config) {
	syncInterval, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		log.WithError(err).Error("Invalid sync interval, using default 6h")
		syncInterval = 6 * time.Hour
	}

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	// Run tasks immediately on startup
	if err := runTasksWithTokenRefresh(ctx, appService, tokenService, &currentToken, repo); err != nil {
		log.WithError(err).Error("Failed to run initial tasks")
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("Background tasks cancelled")
			return
		case <-ticker.C:
			if err := runTasksWithTokenRefresh(ctx, appService, tokenService, &currentToken, repo); err != nil {
				log.WithError(err).Error("Failed to run scheduled tasks")
			}
		}
	}
}

// runTasksWithTokenRefresh runs tasks and handles token refresh with context
func runTasksWithTokenRefresh(ctx context.Context, appService *services.AppService, tokenService *services.TraktTokenService, currentToken **trakt.Token, repo repository.Repository) error {
	// Check if context is cancelled before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Refresh token before running tasks
	refreshedToken, err := tokenService.RefreshToken(*currentToken)
	if err != nil {
		log.WithError(err).Error("Failed to refresh Trakt token, using current token")
	} else {
		*currentToken = refreshedToken
		// Update services with the new token
		traktService := services.NewTraktService(repo, refreshedToken)
		cleanupService := services.NewCleanupService(repo, refreshedToken)
		appService.UpdateTraktServices(traktService, cleanupService)
		log.Debug("Updated Trakt services with refreshed token")
	}

	// Run main application tasks
	if err := appService.RunTasks(); err != nil {
		return fmt.Errorf("running application tasks: %w", err)
	}

	return nil
}

// waitForShutdown waits for shutdown signals and gracefully shuts down
func waitForShutdown(ctx context.Context, cancel context.CancelFunc, server *http.Server, appService *services.AppService, wg *sync.WaitGroup) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.WithField("signal", sig).Info("Received shutdown signal, initiating graceful shutdown")

	// Cancel context to stop background tasks
	cancel()

	// Create context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("Failed to shutdown HTTP server gracefully")
	} else {
		log.Info("HTTP server shut down successfully")
	}

	// Wait for background tasks to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("All goroutines completed successfully")
	case <-shutdownCtx.Done():
		log.Warn("Shutdown timeout reached, forcing exit")
	}

	// Shutdown application service
	if err := appService.Close(); err != nil {
		log.WithError(err).Error("Failed to shutdown application service")
	} else {
		log.Info("Application service shut down successfully")
	}

	log.Info("Graceful shutdown completed")
}
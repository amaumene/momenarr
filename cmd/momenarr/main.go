package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	)
	downloadService := services.NewDownloadService(repo, nzbGetClient, nzbService)
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

	// Start background tasks
	go startBackgroundTasks(appService, tokenService, traktToken, repo, cfg)

	// Start HTTP server
	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.WithField("address", server.Addr).Info("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Wait for shutdown signal
	waitForShutdown(server, appService)
}

// startBackgroundTasks starts the background task loop
func startBackgroundTasks(appService *services.AppService, tokenService *services.TraktTokenService, currentToken *trakt.Token, repo repository.Repository, cfg *config.Config) {
	syncInterval, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		log.WithError(err).Error("Invalid sync interval, using default 6h")
		syncInterval = 6 * time.Hour
	}

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	// Run tasks immediately on startup
	runTasksWithTokenRefresh(appService, tokenService, &currentToken, repo)

	for range ticker.C {
		runTasksWithTokenRefresh(appService, tokenService, &currentToken, repo)
	}
}

// runTasksWithTokenRefresh runs tasks and handles token refresh
func runTasksWithTokenRefresh(appService *services.AppService, tokenService *services.TraktTokenService, currentToken **trakt.Token, repo repository.Repository) {
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
		log.WithError(err).Error("Failed to run application tasks")
	}
}

// waitForShutdown waits for shutdown signals and gracefully shuts down
func waitForShutdown(server *http.Server, appService *services.AppService) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.WithField("signal", sig).Info("Received shutdown signal, initiating graceful shutdown")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Failed to shutdown HTTP server gracefully")
	} else {
		log.Info("HTTP server shut down successfully")
	}

	// Shutdown application service
	if err := appService.Close(); err != nil {
		log.WithError(err).Error("Failed to shutdown application service")
	} else {
		log.Info("Application service shut down successfully")
	}

	log.Info("Graceful shutdown completed")
}
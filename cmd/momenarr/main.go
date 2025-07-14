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

func main() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	log.Info("starting momenarr application with alldebrid support")

	cfg, err := config.LoadNewConfig()
	if err != nil {
		log.WithError(err).Fatal("failed to load configuration")
	}

	if err := cfg.Validate(); err != nil {
		log.WithError(err).Fatal("invalid configuration")
	}

	dbPath := filepath.Join(cfg.DataDir, "data.db")
	store, err := bolthold.Open(dbPath, 0600, nil)
	if err != nil {
		log.WithError(err).Fatal("failed to open database")
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.WithError(err).Error("failed to close database")
		}
	}()

	repo := repository.NewBoltRepository(store)

	// Initialize Trakt
	trakt.Key = cfg.TraktAPIKey
	tokenService := services.NewTraktTokenService(cfg.DataDir, cfg.TraktClientSecret)
	traktToken, err := tokenService.GetToken()
	if err != nil {
		log.WithError(err).Fatal("failed to get trakt token")
	}

	// Initialize Trakt service first
	traktService := services.NewTraktService(repo, traktToken)

	// Initialize AllDebrid and torrent services
	allDebridService := services.NewAllDebridService(
		repo,
		cfg.AllDebridAPIKey,
	)

	torrentService := services.NewTorrentServiceWithTrakt(
		repo,
		cfg.BlacklistFile,
		traktService,
	)

	downloadService := services.CreateNewDownloadService(
		repo,
		allDebridService,
		torrentService,
	)

	cleanupService := services.CreateNewCleanupService(repo, allDebridService, traktToken)
	cleanupService.SetWatchedDays(cfg.WatchedDays)

	// Initialize main application service
	appService := services.CreateNewAppService(
		repo,
		traktService,
		torrentService,
		downloadService,
		cleanupService,
	)

	handler := handlers.CreateNewHandler(appService)
	handler.SetupRoutes()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startBackgroundTasks(ctx, appService, tokenService, traktToken, repo, allDebridService, cfg)
	}()

	server := &http.Server{
		Addr:         cfg.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Start server in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.WithField("address", server.Addr).Info("starting http server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("http server error")
		}
	}()

	log.Info("momenarr is ready")
	log.WithFields(log.Fields{
		"data_dir":      cfg.DataDir,
		"sync_interval": cfg.SyncInterval,
		"watched_days":  cfg.WatchedDays,
	}).Info("configuration loaded")

	waitForShutdown(ctx, cancel, server, appService, &wg)
}

// startBackgroundTasks starts the background task loop with context cancellation
func startBackgroundTasks(ctx context.Context, appService *services.NewAppService, tokenService *services.TraktTokenService, currentToken *trakt.Token, repo repository.Repository, allDebridService *services.AllDebridService, cfg *config.NewConfig) {
	syncInterval, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		log.WithError(err).Error("invalid sync interval, using default 6h")
		syncInterval = 6 * time.Hour
	}

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	// Check for cancellation before running initial tasks
	select {
	case <-ctx.Done():
		log.Info("background tasks cancelled before initial run")
		return
	default:
	}

	if err := runTasksWithTokenRefresh(ctx, appService, tokenService, &currentToken, repo, allDebridService); err != nil {
		// Check if error is due to context cancellation
		if ctx.Err() != nil {
			log.Info("background tasks cancelled during initial run")
			return
		}
		log.WithError(err).Error("failed to run initial tasks")
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("background tasks cancelled")
			return
		case <-ticker.C:
			// Check for cancellation before running scheduled tasks
			select {
			case <-ctx.Done():
				log.Info("background tasks cancelled before scheduled run")
				return
			default:
			}

			if err := runTasksWithTokenRefresh(ctx, appService, tokenService, &currentToken, repo, allDebridService); err != nil {
				// Check if error is due to context cancellation
				if ctx.Err() != nil {
					log.Info("background tasks cancelled during scheduled run")
					return
				}
				log.WithError(err).Error("failed to run scheduled tasks")
			}
		}
	}
}

// runTasksWithTokenRefresh runs tasks and handles token refresh with context
func runTasksWithTokenRefresh(ctx context.Context, appService *services.NewAppService, tokenService *services.TraktTokenService, currentToken **trakt.Token, repo repository.Repository, allDebridService *services.AllDebridService) error {
	// Check if context is cancelled before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Refresh token before running tasks
	refreshedToken, err := tokenService.RefreshToken(*currentToken)
	if err != nil {
		log.WithError(err).Error("failed to refresh trakt token, using current token")
	} else {
		*currentToken = refreshedToken
		// Update services with the new token
		traktService := services.NewTraktService(repo, refreshedToken)
		cleanupService := services.CreateNewCleanupService(repo, allDebridService, refreshedToken)
		appService.UpdateTraktServices(traktService, cleanupService)
		log.Debug("updated trakt services with refreshed token")
	}

	// Run main application tasks with context
	if err := appService.RunTasks(ctx); err != nil {
		return fmt.Errorf("running application tasks: %w", err)
	}

	return nil
}

// waitForShutdown waits for shutdown signals and gracefully shuts down
func waitForShutdown(ctx context.Context, cancel context.CancelFunc, server *http.Server, appService *services.NewAppService, wg *sync.WaitGroup) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	shutdownStart := time.Now()
	log.WithField("signal", sig).Info("received shutdown signal, initiating graceful shutdown")

	// Cancel context to stop background tasks
	cancel()

	// Create context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown components concurrently
	type shutdownResult struct {
		component string
		err       error
	}

	resultChan := make(chan shutdownResult, 2)

	// Start HTTP server shutdown
	go func() {
		log.Info("shutting down http server")
		err := server.Shutdown(shutdownCtx)
		resultChan <- shutdownResult{"HTTP server", err}
	}()

	// Start background tasks shutdown
	go func() {
		log.Info("waiting for background tasks to complete")
		wg.Wait()
		resultChan <- shutdownResult{"Background tasks", nil}
	}()

	// Wait for both to complete or timeout
	var httpDone, tasksDone bool
	for !httpDone || !tasksDone {
		select {
		case result := <-resultChan:
			switch result.component {
			case "HTTP server":
				if !httpDone { // Prevent duplicate logging
					httpDone = true
					if result.err != nil {
						log.WithError(result.err).Error("failed to shutdown http server gracefully")
					} else {
						log.Info("http server shut down successfully")
					}
				}
			case "Background tasks":
				if !tasksDone { // Prevent duplicate logging
					tasksDone = true
					log.Info("all background tasks completed successfully")
				}
			}

		case <-shutdownCtx.Done():
			log.Warn("shutdown timeout reached, forcing exit")
			return // Exit immediately on timeout
		}
	}

	// Shutdown application service
	if err := appService.Close(); err != nil {
		log.WithError(err).Error("failed to shutdown application service")
	} else {
		log.Info("application service shut down successfully")
	}

	shutdownDuration := time.Since(shutdownStart)
	log.WithField("duration", shutdownDuration).Info("graceful shutdown completed")
}

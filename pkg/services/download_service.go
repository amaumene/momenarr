package services

import (
	"context"
	"fmt"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

type DownloadService struct {
	repo             repository.Repository
	allDebridService AllDebridInterface
	torrentService   *TorrentService
}

// CreateDownloadService creates a download service
func CreateDownloadService(repo repository.Repository, allDebrid AllDebridInterface, torrentService *TorrentService) *DownloadService {
	return &DownloadService{
		repo:             repo,
		allDebridService: allDebrid,
		torrentService:   torrentService,
	}
}

// DownloadNotOnDisk downloads all media that is not on disk
func (s *DownloadService) DownloadNotOnDisk() error {
	return s.DownloadNotOnDiskWithContext(context.Background())
}

// DownloadNotOnDiskWithContext downloads all media that is not on disk with context support
func (s *DownloadService) DownloadNotOnDiskWithContext(ctx context.Context) error {
	medias, err := s.repo.FindMediaNotOnDisk()
	if err != nil {
		return fmt.Errorf("finding media not on disk: %w", err)
	}

	log.WithField("count", len(medias)).Info("Processing downloads for media not on disk")

	for _, media := range medias {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.processMediaDownloadWithContext(ctx, media); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"trakt_id": media.Trakt,
				"title":    media.Title,
			}).Error("Failed to process media download")
			continue
		}
	}

	log.Info("Finished processing downloads")
	return nil
}

// processMediaDownloadWithContext processes download for a single media item with context
func (s *DownloadService) processMediaDownloadWithContext(ctx context.Context, media *models.Media) error {
	currentMedia, err := s.repo.GetMedia(media.Trakt)
	if err == nil && currentMedia.OnDisk {
		return nil
	}
	// Search for torrents in real-time and find the best cached one
	bestTorrent, err := s.torrentService.FindBestCachedTorrent(media, s.allDebridService)
	if err != nil {
		return fmt.Errorf("finding best cached torrent: %w", err)
	}

	if bestTorrent == nil {
		log.WithFields(log.Fields{
			"trakt_id": media.Trakt,
			"title":    media.Title,
		}).Info("No cached torrents found")
		return nil
	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
		"hash":     bestTorrent.Hash,
		"seeders":  bestTorrent.Seeders,
		"size_gb":  float64(bestTorrent.Size) / (1024 * 1024 * 1024),
	}).Info("Found cached torrent, downloading")

	// Download the torrent
	success, err := s.downloadBestTorrent(ctx, media, bestTorrent)
	if !success {
		return fmt.Errorf("failed to download best torrent: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
		"hash":     bestTorrent.Hash,
	}).Info("Successfully downloaded torrent")
	return nil
}

// tryTorrent attempts to process a single torrent search result and returns success status
func (s *DownloadService) tryTorrent(ctx context.Context, result *models.TorrentSearchResult, media *models.Media) (bool, error) {
	// First check if the torrent is cached on AllDebrid
	isCached, allDebridID, err := s.allDebridService.IsTorrentCached(result.Hash)
	if err != nil {
		log.WithError(err).WithField("hash", result.Hash).Debug("Failed to check if torrent is cached")
		return false, fmt.Errorf("checking if torrent is cached: %w", err)
	}

	if !isCached {
		log.WithFields(log.Fields{
			"hash":     result.Hash,
			"title":    result.Title,
			"size_gb":  fmt.Sprintf("%.2f", float64(result.Size)/(1024*1024*1024)),
			"trakt_id": media.Trakt,
			"media":    media.Title,
		}).Info("Torrent not cached on AllDebrid")
		return false, fmt.Errorf("torrent not cached on AllDebrid")
	}

	// Torrent is cached! No need to save to database since we're not storing torrents

	log.WithFields(log.Fields{
		"hash":         result.Hash,
		"title":        result.Title,
		"trakt_id":     media.Trakt,
		"alldebrid_id": allDebridID,
	}).Info("Torrent cached on AllDebrid - marking as completed")

	// Mark media as "on disk" since the torrent is cached on AllDebrid
	media.OnDisk = true
	media.File = fmt.Sprintf("AllDebrid magnet ID: %d", allDebridID)
	if saveErr := s.repo.SaveMedia(media); saveErr != nil {
		log.WithError(saveErr).Error("Failed to save media as completed")
		return false, fmt.Errorf("saving media as completed: %w", saveErr)
	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
		"torrent":  result.Title,
	}).Info("Successfully processed cached torrent")

	return true, nil
}

// RetryFailedDownload retries a failed download
func (s *DownloadService) RetryFailedDownload(traktID int64) error {
	// Get media and try to download again
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("getting media for retry: %w", err)
	}

	// Reset OnDisk status to allow retry
	media.OnDisk = false
	if err := s.repo.SaveMedia(media); err != nil {
		log.WithError(err).Error("Failed to reset media OnDisk status for retry")
	}

	return s.processMediaDownloadWithContext(context.Background(), media)
}

// GetDownloadStatus gets the status of a download
func (s *DownloadService) GetDownloadStatus(traktID int64) (string, error) {
	// Check media status instead of torrent since torrents are no longer stored
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return "NOT_FOUND", nil
	}

	// Since we don't store torrents anymore, check media status directly
	if media.OnDisk {
		return "COMPLETED", nil
	}

	return "NOT_STARTED", nil
}

// CancelDownload cancels a download
func (s *DownloadService) CancelDownload(traktID int64) error {
	// Since we don't store torrents anymore, just reset the media OnDisk status
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("getting media: %w", err)
	}

	media.OnDisk = false
	media.File = ""
	if err := s.repo.SaveMedia(media); err != nil {
		return fmt.Errorf("resetting media status: %w", err)
	}

	log.WithField("trakt_id", traktID).Info("Download canceled - media marked as not on disk")
	return nil
}

// downloadBestTorrent downloads the best torrent found from real-time search
func (s *DownloadService) downloadBestTorrent(ctx context.Context, media *models.Media, torrent *models.TorrentSearchResult) (bool, error) {
	// Check if torrent is cached on AllDebrid (double-check)
	isCached, allDebridID, err := s.allDebridService.IsTorrentCached(torrent.Hash)
	if err != nil {
		return false, fmt.Errorf("checking if torrent is cached: %w", err)
	}

	if !isCached {
		log.WithFields(log.Fields{
			"hash":     torrent.Hash,
			"title":    torrent.Title,
			"size_gb":  fmt.Sprintf("%.2f", float64(torrent.Size)/(1024*1024*1024)),
			"trakt_id": media.Trakt,
			"media":    media.Title,
		}).Warn("Torrent not cached on AllDebrid (cache status changed)")
		return false, fmt.Errorf("torrent not cached on AllDebrid")
	}

	log.WithFields(log.Fields{
		"hash":         torrent.Hash,
		"title":        torrent.Title,
		"alldebrid_id": allDebridID,
		"size_gb":      fmt.Sprintf("%.2f", float64(torrent.Size)/(1024*1024*1024)),
		"trakt_id":     media.Trakt,
		"media":        media.Title,
	}).Info("Torrent cached on AllDebrid, marking as downloaded")

	// Mark media as downloaded and save AllDebrid ID
	media.OnDisk = true
	media.File = fmt.Sprintf("AllDebrid magnet ID: %d", allDebridID)
	if saveErr := s.repo.SaveMedia(media); saveErr != nil {
		log.WithError(saveErr).Error("Failed to save media as completed")
		return false, fmt.Errorf("saving media as completed: %w", saveErr)
	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
		"torrent":  torrent.Title,
	}).Info("Successfully processed cached torrent")

	return true, nil
}

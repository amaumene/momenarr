package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

type NewDownloadService struct {
	repo             repository.Repository
	allDebridService *AllDebridService
	torrentService   *TorrentService
	processingMedia  map[int64]bool
	processingMutex  sync.RWMutex
}

// CreateNewDownloadService creates a new download service
func CreateNewDownloadService(repo repository.Repository, allDebrid *AllDebridService, torrentService *TorrentService) *NewDownloadService {
	return &NewDownloadService{
		repo:             repo,
		allDebridService: allDebrid,
		torrentService:   torrentService,
		processingMedia:  make(map[int64]bool),
	}
}

// DownloadNotOnDisk downloads all media that is not on disk
func (s *NewDownloadService) DownloadNotOnDisk() error {
	return s.DownloadNotOnDiskWithContext(context.Background())
}

// DownloadNotOnDiskWithContext downloads all media that is not on disk with context support
func (s *NewDownloadService) DownloadNotOnDiskWithContext(ctx context.Context) error {
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
func (s *NewDownloadService) processMediaDownloadWithContext(ctx context.Context, media *models.Media) error {
	s.processingMutex.Lock()
	if s.processingMedia[media.Trakt] {
		s.processingMutex.Unlock()
		return nil
	}
	s.processingMedia[media.Trakt] = true
	s.processingMutex.Unlock()

	defer func() {
		s.processingMutex.Lock()
		delete(s.processingMedia, media.Trakt)
		s.processingMutex.Unlock()
	}()

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

// tryTorrent attempts to process a single torrent and returns success status
func (s *NewDownloadService) tryTorrent(ctx context.Context, torrent *models.Torrent, media *models.Media) (bool, error) {
	// First check if the torrent is cached on AllDebrid
	isCached, allDebridID, err := s.allDebridService.IsTorrentCached(torrent.Hash)
	if err != nil {
		log.WithError(err).WithField("hash", torrent.Hash).Debug("Failed to check if torrent is cached")
		return false, fmt.Errorf("checking if torrent is cached: %w", err)
	}

	if !isCached {
		log.WithFields(log.Fields{
			"hash":     torrent.Hash,
			"title":    torrent.Title,
			"size_gb":  fmt.Sprintf("%.2f", float64(torrent.Size)/(1024*1024*1024)),
			"trakt_id": media.Trakt,
			"media":    media.Title,
		}).Info("Torrent not cached on AllDebrid")
		return false, fmt.Errorf("torrent not cached on AllDebrid")
	}

	// Torrent is cached! Save the AllDebrid ID and mark as done
	torrent.AllDebridID = allDebridID
	if err := s.repo.SaveTorrent(torrent); err != nil {
		log.WithError(err).Error("Failed to save torrent with AllDebrid ID")
		return false, fmt.Errorf("saving torrent with AllDebrid ID: %w", err)
	}

	log.WithFields(log.Fields{
		"hash":         torrent.Hash,
		"title":        torrent.Title,
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
		"torrent":  torrent.Title,
	}).Info("Successfully processed cached torrent")

	return true, nil
}

// RetryFailedDownload retries a failed download
func (s *NewDownloadService) RetryFailedDownload(traktID int64) error {
	// Mark current torrent as failed
	if err := s.torrentService.MarkTorrentFailed(traktID); err != nil {
		log.WithError(err).WithField("trakt_id", traktID).Error("Failed to mark torrent as failed")
	}

	// Get media and try to download again
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("getting media for retry: %w", err)
	}

	return s.processMediaDownloadWithContext(context.Background(), media)
}

// GetDownloadStatus gets the status of a download
func (s *NewDownloadService) GetDownloadStatus(traktID int64) (string, error) {
	// Get the torrent
	torrent, err := s.torrentService.GetBestTorrent(traktID)
	if err != nil {
		return "NOT_FOUND", nil
	}

	if torrent.AllDebridID == 0 {
		return "NOT_STARTED", nil
	}

	// Check AllDebrid status
	status, err := s.allDebridService.client.GetMagnetStatus(s.allDebridService.apiKey, []int64{torrent.AllDebridID})
	if err != nil {
		return "ERROR", err
	}

	if status.Status != "success" || len(status.Data.Magnets) == 0 {
		return "ERROR", nil
	}

	magnet := status.Data.Magnets[0]
	if magnet.Ready {
		// Check if on disk
		media, err := s.repo.GetMedia(traktID)
		if err == nil && media.OnDisk {
			return "COMPLETED", nil
		}
		return "READY", nil
	}

	return magnet.Status, nil
}

// CancelDownload cancels a download
func (s *NewDownloadService) CancelDownload(traktID int64) error {
	// Get the torrent
	torrent, err := s.torrentService.GetBestTorrent(traktID)
	if err != nil {
		return fmt.Errorf("getting torrent: %w", err)
	}

	if torrent.AllDebridID == 0 {
		return fmt.Errorf("no active download for Trakt ID %d", traktID)
	}

	// Delete from AllDebrid
	if err := s.allDebridService.DeleteMagnet(torrent.AllDebridID); err != nil {
		return fmt.Errorf("deleting from AllDebrid: %w", err)
	}

	// Mark torrent as failed
	torrent.MarkFailed()
	if err := s.repo.SaveTorrent(torrent); err != nil {
		return fmt.Errorf("updating torrent: %w", err)
	}

	log.WithField("trakt_id", traktID).Info("Download canceled")
	return nil
}

// downloadBestTorrent downloads the best torrent found from real-time search
func (s *NewDownloadService) downloadBestTorrent(ctx context.Context, media *models.Media, torrent *models.TorrentSearchResult) (bool, error) {
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

package services

import (
	"context"
	"fmt"
	"strconv"

	"github.com/amaumene/gostremiofr/pkg/alldebrid"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

type DownloadService struct {
	repo             repository.Repository
	allDebridClient  *alldebrid.Client
	apiKey           string
	torrentService   *TorrentService
}

// CreateDownloadService creates a download service
func CreateDownloadService(repo repository.Repository, allDebridClient *alldebrid.Client, apiKey string, torrentService *TorrentService) *DownloadService {
	return &DownloadService{
		repo:             repo,
		allDebridClient:  allDebridClient,
		apiKey:           apiKey,
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
	s.processAllMedia(ctx, medias)
	log.Info("Finished processing downloads")
	return nil
}

// processAllMedia processes downloads for all media items
func (s *DownloadService) processAllMedia(ctx context.Context, medias []*models.Media) {
	for _, media := range medias {
		if s.shouldStopProcessing(ctx) {
			return
		}

		s.processSingleMedia(ctx, media)
	}
}

// shouldStopProcessing checks if processing should stop
func (s *DownloadService) shouldStopProcessing(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// processSingleMedia processes a single media download
func (s *DownloadService) processSingleMedia(ctx context.Context, media *models.Media) {
	if err := s.processMediaDownloadWithContext(ctx, media); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"trakt_id": media.Trakt,
			"title":    media.Title,
		}).Error("Failed to process media download")
	}
}

// processMediaDownloadWithContext processes download for a single media item with context
func (s *DownloadService) processMediaDownloadWithContext(ctx context.Context, media *models.Media) error {
	if s.isMediaAlreadyOnDisk(media) {
		return nil
	}

	bestTorrent, err := s.findCachedTorrent(media)
	if err != nil {
		return err
	}

	if bestTorrent == nil {
		s.logNoCachedTorrents(media)
		return nil
	}

	s.logFoundCachedTorrent(media, bestTorrent)
	return s.downloadAndLog(ctx, media, bestTorrent)
}

// isMediaAlreadyOnDisk checks if media is already on disk
func (s *DownloadService) isMediaAlreadyOnDisk(media *models.Media) bool {
	currentMedia, err := s.repo.GetMedia(media.Trakt)
	return err == nil && currentMedia.OnDisk
}

// findCachedTorrent searches for the best cached torrent
func (s *DownloadService) findCachedTorrent(media *models.Media) (*models.TorrentSearchResult, error) {
	bestTorrent, err := s.torrentService.FindBestCachedTorrent(media, s.allDebridClient, s.apiKey)
	if err != nil {
		return nil, fmt.Errorf("finding best cached torrent: %w", err)
	}
	return bestTorrent, nil
}

// logNoCachedTorrents logs when no cached torrents are found
func (s *DownloadService) logNoCachedTorrents(media *models.Media) {
	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
	}).Info("No cached torrents found")
}

// logFoundCachedTorrent logs when a cached torrent is found
func (s *DownloadService) logFoundCachedTorrent(media *models.Media, torrent *models.TorrentSearchResult) {
	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
		"hash":     torrent.Hash,
		"seeders":  torrent.Seeders,
		"size_gb":  float64(torrent.Size) / (1024 * 1024 * 1024),
	}).Info("Found cached torrent, downloading")
}

// downloadAndLog downloads torrent and logs the result
func (s *DownloadService) downloadAndLog(ctx context.Context, media *models.Media, torrent *models.TorrentSearchResult) error {
	success, err := s.downloadBestTorrent(ctx, media, torrent)
	if !success {
		return fmt.Errorf("failed to download best torrent: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
		"hash":     torrent.Hash,
	}).Info("Successfully downloaded torrent")
	return nil
}

// tryTorrent attempts to process a single torrent search result and returns success status
func (s *DownloadService) tryTorrent(ctx context.Context, result *models.TorrentSearchResult, media *models.Media) (bool, error) {
	isCached, allDebridID, err := s.checkTorrentCache(result)
	if err != nil {
		return false, err
	}

	if !isCached {
		s.logTorrentNotCached(result, media)
		return false, fmt.Errorf("torrent not cached on AllDebrid")
	}

	s.logCachedTorrent(result, media, allDebridID)
	return s.markMediaAsCompleted(media, allDebridID, result)
}

// checkTorrentCache checks if torrent is cached on AllDebrid
func (s *DownloadService) checkTorrentCache(result *models.TorrentSearchResult) (bool, int64, error) {
	isCached, allDebridID, err := s.isTorrentCached(result.Hash)
	if err != nil {
		log.WithError(err).WithField("hash", result.Hash).Debug("Failed to check if torrent is cached")
		return false, 0, fmt.Errorf("checking if torrent is cached: %w", err)
	}
	return isCached, allDebridID, nil
}

// logTorrentNotCached logs when torrent is not cached
func (s *DownloadService) logTorrentNotCached(result *models.TorrentSearchResult, media *models.Media) {
	log.WithFields(log.Fields{
		"hash":     result.Hash,
		"title":    result.Title,
		"size_gb":  fmt.Sprintf("%.2f", float64(result.Size)/(1024*1024*1024)),
		"trakt_id": media.Trakt,
		"media":    media.Title,
	}).Info("Torrent not cached on AllDebrid")
}

// logCachedTorrent logs when torrent is cached
func (s *DownloadService) logCachedTorrent(result *models.TorrentSearchResult, media *models.Media, allDebridID int64) {
	log.WithFields(log.Fields{
		"hash":         result.Hash,
		"title":        result.Title,
		"trakt_id":     media.Trakt,
		"alldebrid_id": allDebridID,
	}).Info("Torrent cached on AllDebrid - marking as completed")
}

// markMediaAsCompleted marks media as completed and saves to database
func (s *DownloadService) markMediaAsCompleted(media *models.Media, allDebridID int64, result *models.TorrentSearchResult) (bool, error) {
	media.OnDisk = true
	media.File = fmt.Sprintf("AllDebrid magnet ID: %d", allDebridID)
	media.MagnetID = fmt.Sprintf("%d", allDebridID)
	
	if result.IsSeasonPack() {
		media.IsSeasonPack = true
		media.SeasonPackID = allDebridID
	}

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
	isCached, allDebridID, err := s.verifyTorrentCached(torrent)
	if err != nil {
		return false, err
	}

	if !isCached {
		s.logCacheStatusChanged(torrent, media)
		return false, fmt.Errorf("torrent not cached on AllDebrid")
	}

	s.logDownloadingTorrent(torrent, media, allDebridID)
	return s.saveTorrentAsDownloaded(media, allDebridID, torrent)
}

// verifyTorrentCached double-checks if torrent is cached
func (s *DownloadService) verifyTorrentCached(torrent *models.TorrentSearchResult) (bool, int64, error) {
	isCached, allDebridID, err := s.isTorrentCached(torrent.Hash)
	if err != nil {
		return false, 0, fmt.Errorf("checking if torrent is cached: %w", err)
	}
	return isCached, allDebridID, nil
}

// logCacheStatusChanged logs when cache status has changed
func (s *DownloadService) logCacheStatusChanged(torrent *models.TorrentSearchResult, media *models.Media) {
	log.WithFields(log.Fields{
		"hash":     torrent.Hash,
		"title":    torrent.Title,
		"size_gb":  fmt.Sprintf("%.2f", float64(torrent.Size)/(1024*1024*1024)),
		"trakt_id": media.Trakt,
		"media":    media.Title,
	}).Warn("Torrent not cached on AllDebrid (cache status changed)")
}

// logDownloadingTorrent logs torrent download
func (s *DownloadService) logDownloadingTorrent(torrent *models.TorrentSearchResult, media *models.Media, allDebridID int64) {
	log.WithFields(log.Fields{
		"hash":         torrent.Hash,
		"title":        torrent.Title,
		"alldebrid_id": allDebridID,
		"size_gb":      fmt.Sprintf("%.2f", float64(torrent.Size)/(1024*1024*1024)),
		"trakt_id":     media.Trakt,
		"media":        media.Title,
	}).Info("Torrent cached on AllDebrid, marking as downloaded")
}

// saveTorrentAsDownloaded saves torrent as downloaded
func (s *DownloadService) saveTorrentAsDownloaded(media *models.Media, allDebridID int64, torrent *models.TorrentSearchResult) (bool, error) {
	media.OnDisk = true
	media.File = fmt.Sprintf("AllDebrid magnet ID: %d", allDebridID)
	media.MagnetID = fmt.Sprintf("%d", allDebridID)
	
	if torrent.IsSeasonPack() {
		media.IsSeasonPack = true
		media.SeasonPackID = allDebridID
	}

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

// isTorrentCached checks if a torrent is cached on AllDebrid
func (s *DownloadService) isTorrentCached(hash string) (bool, int64, error) {
	magnetURL := fmt.Sprintf("magnet:?xt=urn:btih:%s", hash)
	
	uploadResult, err := s.allDebridClient.UploadMagnet(s.apiKey, []string{magnetURL})
	if err != nil {
		return false, 0, fmt.Errorf("failed to upload magnet: %w", err)
	}
	
	if uploadResult.Error != nil {
		return false, 0, fmt.Errorf("upload error: %s", uploadResult.Error.Message)
	}
	
	if len(uploadResult.Data.Magnets) == 0 {
		return false, 0, nil
	}
	
	magnet := &uploadResult.Data.Magnets[0]
	if magnet.Error != nil {
		return false, 0, fmt.Errorf("magnet error: %s", magnet.Error.Message)
	}
	
	if magnet.Ready {
		return true, int64(magnet.ID), nil
	}
	
	// If not ready, delete it
	if err := s.allDebridClient.DeleteMagnet(s.apiKey, strconv.FormatInt(magnet.ID, 10)); err != nil {
		log.WithError(err).Error("Failed to delete non-cached magnet")
	}
	
	return false, 0, nil
}

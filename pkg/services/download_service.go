package services

import (
	"context"
	"fmt"

	"github.com/amaumene/gostremiofr/pkg/alldebrid"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

type DownloadService struct {
	repo             repository.Repository
	allDebridService *AllDebridService
	torrentService   *TorrentService
}

func CreateDownloadService(repo repository.Repository, allDebridClient *alldebrid.Client, apiKey string, torrentService *TorrentService) *DownloadService {
	return &DownloadService{
		repo:             repo,
		allDebridService: NewAllDebridService(allDebridClient, apiKey),
		torrentService:   torrentService,
	}
}

func (s *DownloadService) DownloadNotOnDisk() error {
	return s.DownloadNotOnDiskWithContext(context.Background())
}

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

func (s *DownloadService) processAllMedia(ctx context.Context, medias []*models.Media) {
	for _, media := range medias {
		if s.shouldStopProcessing(ctx) {
			return
		}

		s.processSingleMedia(ctx, media)
	}
}

func (s *DownloadService) shouldStopProcessing(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (s *DownloadService) processSingleMedia(ctx context.Context, media *models.Media) {
	if err := s.processMediaDownloadWithContext(ctx, media); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"trakt_id": media.Trakt,
			"title":    media.Title,
		}).Error("Failed to process media download")
	}
}

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

func (s *DownloadService) isMediaAlreadyOnDisk(media *models.Media) bool {
	currentMedia, err := s.repo.GetMedia(media.Trakt)
	return err == nil && currentMedia.OnDisk
}

func (s *DownloadService) findCachedTorrent(media *models.Media) (*models.TorrentSearchResult, error) {
	bestTorrent, err := s.torrentService.FindBestCachedTorrent(media)
	if err != nil {
		return nil, fmt.Errorf("finding best cached torrent: %w", err)
	}
	return bestTorrent, nil
}

func (s *DownloadService) logNoCachedTorrents(media *models.Media) {
	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
	}).Info("No cached torrents found")
}

func (s *DownloadService) logFoundCachedTorrent(media *models.Media, torrent *models.TorrentSearchResult) {
	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    media.Title,
		"hash":     torrent.Hash,
		"seeders":  torrent.Seeders,
		"size_gb":  float64(torrent.Size) / (1024 * 1024 * 1024),
	}).Info("Found cached torrent, downloading")
}

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

func (s *DownloadService) checkTorrentCache(result *models.TorrentSearchResult) (bool, int64, error) {
	isCached, allDebridID, err := s.allDebridService.IsTorrentCached(result.Hash)
	if err != nil {
		log.WithError(err).WithField("hash", result.Hash).Debug("Failed to check if torrent is cached")
		return false, 0, fmt.Errorf("checking if torrent is cached: %w", err)
	}
	return isCached, allDebridID, nil
}

func (s *DownloadService) logTorrentNotCached(result *models.TorrentSearchResult, media *models.Media) {
	log.WithFields(log.Fields{
		"hash":     result.Hash,
		"title":    result.Title,
		"size_gb":  fmt.Sprintf("%.2f", float64(result.Size)/(1024*1024*1024)),
		"trakt_id": media.Trakt,
		"media":    media.Title,
	}).Info("Torrent not cached on AllDebrid")
}

func (s *DownloadService) logCachedTorrent(result *models.TorrentSearchResult, media *models.Media, allDebridID int64) {
	log.WithFields(log.Fields{
		"hash":         result.Hash,
		"title":        result.Title,
		"trakt_id":     media.Trakt,
		"alldebrid_id": allDebridID,
	}).Info("Torrent cached on AllDebrid - marking as completed")
}

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

func (s *DownloadService) RetryFailedDownload(traktID int64) error {
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("getting media for retry: %w", err)
	}

	media.OnDisk = false
	if err := s.repo.SaveMedia(media); err != nil {
		log.WithError(err).Error("Failed to reset media OnDisk status for retry")
	}

	return s.processMediaDownloadWithContext(context.Background(), media)
}

func (s *DownloadService) GetDownloadStatus(traktID int64) (string, error) {
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return "NOT_FOUND", nil
	}

	if media.OnDisk {
		return "COMPLETED", nil
	}

	return "NOT_STARTED", nil
}

func (s *DownloadService) CancelDownload(traktID int64) error {
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

func (s *DownloadService) verifyTorrentCached(torrent *models.TorrentSearchResult) (bool, int64, error) {
	isCached, allDebridID, err := s.allDebridService.IsTorrentCached(torrent.Hash)
	if err != nil {
		return false, 0, fmt.Errorf("checking if torrent is cached: %w", err)
	}
	return isCached, allDebridID, nil
}

func (s *DownloadService) logCacheStatusChanged(torrent *models.TorrentSearchResult, media *models.Media) {
	log.WithFields(log.Fields{
		"hash":     torrent.Hash,
		"title":    torrent.Title,
		"size_gb":  fmt.Sprintf("%.2f", float64(torrent.Size)/(1024*1024*1024)),
		"trakt_id": media.Trakt,
		"media":    media.Title,
	}).Warn("Torrent not cached on AllDebrid (cache status changed)")
}

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

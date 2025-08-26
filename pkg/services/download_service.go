package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/premiumize"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

type DownloadService struct {
	repo             repository.Repository
	premiumizeClient *premiumize.Client
	nzbService       *NZBService
	httpClient       *http.Client
	folderID         string
}

func NewDownloadService(repo repository.Repository, premiumizeClient *premiumize.Client, nzbService *NZBService) *DownloadService {
	return &DownloadService{
		repo:             repo,
		premiumizeClient: premiumizeClient,
		nzbService:       nzbService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 50,
				MaxConnsPerHost:     50,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		folderID: "",
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

	for _, media := range medias {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.processMediaDownloadWithContext(ctx, media); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"trakt": media.Trakt,
				"title": media.Title,
			}).Error("Failed to process media download")
			continue
		}
	}

	log.Info("Finished processing downloads")
	return nil
}

func (s *DownloadService) processMediaDownload(media *models.Media) error {
	return s.processMediaDownloadWithContext(context.Background(), media)
}

func (s *DownloadService) processMediaDownloadWithContext(ctx context.Context, media *models.Media) error {
	if media.TransferID != "" {
		if err := s.checkAndUpdateTransferStatus(media); err != nil {
			log.WithError(err).WithField("trakt", media.Trakt).Warn("Failed to check transfer status")
		} else {
			return nil
		}
	}

	nzb, err := s.nzbService.GetNZBFromDB(media.Trakt)
	if err != nil {
		return fmt.Errorf("getting NZB from database: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt":       media.Trakt,
		"media_title": media.Title,
		"nzb_title":   nzb.Title,
		"size_gb":     float64(nzb.Length) / (1024 * 1024 * 1024),
	}).Info("Selected NZB for download")

	if err := s.CreateDownloadWithContext(ctx, media.Trakt, nzb); err != nil {
		if strings.Contains(err.Error(), "already added this nzb file") {
			log.WithFields(log.Fields{
				"trakt": media.Trakt,
				"title": media.Title,
			}).Info("NZB already added to Premiumize, skipping")
			return nil
		}
		return fmt.Errorf("creating download: %w", err)
	}

	return nil
}

func (s *DownloadService) CreateDownload(traktID int64, nzb *models.NZB) error {
	return s.CreateDownloadWithContext(context.Background(), traktID, nzb)
}

func (s *DownloadService) CreateDownloadWithContext(ctx context.Context, traktID int64, nzb *models.NZB) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := s.checkQueueStatus(traktID, nzb); err != nil {
		return err
	}

	transfer, err := s.createPremiumizeTransfer(ctx, nzb)
	if err != nil {
		return err
	}

	return s.updateMediaWithTransfer(ctx, traktID, nzb, transfer)
}

func (s *DownloadService) checkQueueStatus(traktID int64, nzb *models.NZB) error {
	exists, err := s.isAlreadyInQueue(nzb.Title)
	if err != nil {
		return fmt.Errorf("checking if NZB is already in queue: %w", err)
	}
	if exists {
		log.WithFields(log.Fields{
			"trakt": traktID,
			"title": nzb.Title,
		}).Info("NZB already in queue, skipping")
		return fmt.Errorf("already in queue")
	}
	return nil
}

func (s *DownloadService) createPremiumizeTransfer(ctx context.Context, nzb *models.NZB) (*premiumize.Transfer, error) {
	nzbData, err := s.downloadNZBContent(ctx, nzb.Link)
	if err != nil {
		return nil, fmt.Errorf("downloading NZB content: %w", err)
	}

	filename := s.extractNZBFilename(nzb)
	transfer, err := s.premiumizeClient.CreateTransferWithFilename(ctx, nzbData, filename, s.folderID)
	if err != nil {
		return nil, fmt.Errorf("creating Premiumize transfer: %w", err)
	}

	return transfer, nil
}

func (s *DownloadService) updateMediaWithTransfer(ctx context.Context, traktID int64, nzb *models.NZB, transfer *premiumize.Transfer) error {
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("getting media: %w", err)
	}

	downloadID := s.convertTransferID(transfer.ID)
	media.TransferID = transfer.ID
	media.DownloadID = downloadID
	media.UpdatedAt = time.Now()

	if isSeasonPack(nzb.Title) {
		media.IsSeasonPack = true
		if err := s.createSeasonPackRecord(media, transfer.ID); err != nil {
			log.WithError(err).WithField("trakt", traktID).Error("Failed to create season pack record")
		}
	}

	if err := s.repo.SaveMedia(media); err != nil {
		log.WithError(err).WithFields(log.Fields{
			"trakt":       traktID,
			"transfer_id": transfer.ID,
		}).Error("Failed to update media with transfer ID")
		return fmt.Errorf("updating media: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt":         traktID,
		"title":         nzb.Title,
		"transfer_id":   transfer.ID,
		"download_id":   downloadID,
		"is_season_pack": media.IsSeasonPack,
	}).Info("Download started successfully and media updated")

	return nil
}

func (s *DownloadService) convertTransferID(transferID string) int64 {
	downloadID, err := strconv.ParseInt(transferID, 10, 64)
	if err != nil {
		downloadID = int64(hashString(transferID))
	}
	return downloadID
}

func (s *DownloadService) checkAndUpdateTransferStatus(media *models.Media) error {
	transfer, err := s.premiumizeClient.GetTransfer(context.Background(), media.TransferID)
	if err != nil {
		if err == premiumize.ErrTransferNotFound {
			media.TransferID = ""
			media.DownloadID = 0
			return s.repo.SaveMedia(media)
		}
		return fmt.Errorf("checking transfer status: %w", err)
	}

	if transfer.Status.IsComplete() {
		if !media.OnDisk {
			media.OnDisk = true
			media.File = transfer.FileID
			media.UpdatedAt = time.Now()
			if err := s.repo.SaveMedia(media); err != nil {
				return fmt.Errorf("updating media as on disk: %w", err)
			}
			log.WithFields(log.Fields{
				"trakt":       media.Trakt,
				"title":       media.Title,
				"transfer_id": media.TransferID,
			}).Info("Transfer complete, marked as on disk")
		}
		return nil
	}
	
	if transfer.Status.IsActive() {
		log.WithFields(log.Fields{
			"trakt":       media.Trakt,
			"title":       media.Title,
			"transfer_id": media.TransferID,
			"status":      transfer.Status,
			"progress":    transfer.Progress,
		}).Info("Transfer still in progress")
		return nil
	}
	
	if transfer.Status.IsFailed() {
		media.TransferID = ""
		media.DownloadID = 0
		if err := s.repo.SaveMedia(media); err != nil {
			return fmt.Errorf("clearing failed transfer: %w", err)
		}
		return fmt.Errorf("transfer failed, will retry")
	}
	
	return nil
}

func (s *DownloadService) isAlreadyInQueue(title string) (bool, error) {
	transfers, err := s.premiumizeClient.GetTransfers(context.Background())
	if err != nil {
		return false, fmt.Errorf("getting Premiumize transfers: %w", err)
	}

	for _, transfer := range transfers {
		if transfer.Name == title && transfer.Status.IsActive() {
			return true, nil
		}
	}

	return false, nil
}

func (s *DownloadService) downloadNZBContent(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading NZB file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download NZB file, status: %s", resp.Status)
	}

	const maxNZBSize = 50 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxNZBSize)

	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("reading NZB file content: %w", err)
	}

	return content, nil
}

func (s *DownloadService) extractNZBFilename(nzb *models.NZB) string {
	sanitizedTitle := strings.ReplaceAll(nzb.Title, " ", ".")
	sanitizedTitle = regexp.MustCompile(`[^a-zA-Z0-9._-]`).ReplaceAllString(sanitizedTitle, "")
	
	if !strings.HasSuffix(strings.ToLower(sanitizedTitle), ".nzb") {
		sanitizedTitle += ".nzb"
	}
	
	return sanitizedTitle
}

func hashString(s string) uint32 {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

func isSeasonPack(title string) bool {
	lowerTitle := strings.ToLower(title)
	seasonPackIndicators := []string{
		"season",
		"complete",
		"s01-", "s02-", "s03-", "s04-", "s05-",
		"s06-", "s07-", "s08-", "s09-", "s10-",
		"1080p.web", "2160p.web",
	}
	
	for _, indicator := range seasonPackIndicators {
		if strings.Contains(lowerTitle, indicator) {
			return true
		}
	}
	
	if regexp.MustCompile(`s\d{2}e\d{2}-e\d{2}`).MatchString(lowerTitle) {
		return true
	}
	
	return false
}

func (s *DownloadService) createSeasonPackRecord(media *models.Media, transferID string) error {
	if !media.IsEpisode() {
		return nil
	}
	
	pack := s.buildSeasonPack(media, transferID)
	s.linkEpisodesToPack(pack, media.ShowIMDBID, media.Season)
	
	return s.repo.SaveSeasonPack(pack)
}

func (s *DownloadService) buildSeasonPack(media *models.Media, transferID string) *models.SeasonPack {
	return &models.SeasonPack{
		ID:         time.Now().UnixNano(),
		ShowIMDBID: media.ShowIMDBID,
		ShowTitle:  media.ShowTitle,
		Season:     media.Season,
		TransferID: transferID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func (s *DownloadService) linkEpisodesToPack(pack *models.SeasonPack, showIMDBID string, season int64) {
	episodes, err := s.repo.GetEpisodesBySeason(showIMDBID, season)
	if err != nil {
		return
	}
	
	pack.TotalEpisodes = len(episodes)
	for _, ep := range episodes {
		pack.Episodes = append(pack.Episodes, ep.Number)
		ep.IsSeasonPack = true
		ep.SeasonPackID = pack.ID
		s.repo.SaveMedia(ep)
	}
}

func (s *DownloadService) GetDownloadStatus(downloadID int64) (string, error) {
	transferID := strconv.FormatInt(downloadID, 10)
	
	transfer, err := s.premiumizeClient.GetTransfer(context.Background(), transferID)
	if err != nil {
		if err == premiumize.ErrTransferNotFound {
			return "NOT_FOUND", nil
		}
		return "", fmt.Errorf("getting transfer status: %w", err)
	}

	return string(transfer.Status), nil
}

func (s *DownloadService) CancelDownload(downloadID int64) error {
	transferID := strconv.FormatInt(downloadID, 10)
	
	if err := s.premiumizeClient.DeleteTransfer(context.Background(), transferID); err != nil {
		return fmt.Errorf("canceling download: %w", err)
	}

	log.WithField("download_id", downloadID).Info("Download canceled")
	return nil
}

func (s *DownloadService) RetryFailedDownload(traktID int64) error {
	if err := s.nzbService.MarkNZBFailed(traktID); err != nil {
		log.WithError(err).WithField("trakt", traktID).Error("Failed to mark NZB as failed")
	}

	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("getting media for retry: %w", err)
	}

	return s.processMediaDownload(media)
}

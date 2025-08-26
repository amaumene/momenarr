package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/premiumize"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

type PremiumizeMonitorService struct {
	repo             repository.Repository
	premiumizeClient *premiumize.Client
	downloadManager  *premiumize.DownloadManager
}

func NewPremiumizeMonitorService(repo repository.Repository, premiumizeClient *premiumize.Client) *PremiumizeMonitorService {
	return &PremiumizeMonitorService{
		repo:             repo,
		premiumizeClient: premiumizeClient,
		downloadManager:  premiumize.NewDownloadManager(premiumizeClient),
	}
}

func (s *PremiumizeMonitorService) MonitorTransfers(ctx context.Context) error {
	transfers, err := s.premiumizeClient.GetTransfers(ctx)
	if err != nil {
		return fmt.Errorf("getting transfers: %w", err)
	}

	for _, transfer := range transfers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.processTransfer(ctx, &transfer); err != nil {
			log.WithError(err).WithField("transfer_id", transfer.ID).Error("Failed to process transfer")
			continue
		}
	}

	return nil
}

func (s *PremiumizeMonitorService) processTransfer(ctx context.Context, transfer *premiumize.Transfer) error {
	media, err := s.findMediaForTransfer(transfer)
	if err != nil || media == nil {
		return nil
	}

	s.logTransferProgress(transfer, media)

	switch {
	case transfer.Status.IsComplete():
		return s.handleCompletedTransfer(ctx, transfer, media)
	case transfer.Status.IsFailed():
		return s.handleFailedTransfer(ctx, transfer, media)
	}

	return nil
}

func (s *PremiumizeMonitorService) findMediaForTransfer(transfer *premiumize.Transfer) (*models.Media, error) {
	downloadID, err := strconv.ParseInt(transfer.ID, 10, 64)
	if err != nil {
		downloadID = int64(s.hashTransferID(transfer.ID))
	}
	return s.repo.GetMediaByDownloadID(downloadID)
}

func (s *PremiumizeMonitorService) logTransferProgress(transfer *premiumize.Transfer, media *models.Media) {
	log.WithFields(log.Fields{
		"transfer_id": transfer.ID,
		"status":      transfer.Status,
		"progress":    transfer.Progress,
		"trakt":       media.Trakt,
		"title":       media.Title,
	}).Debug("Processing transfer")
}

func (s *PremiumizeMonitorService) hashTransferID(id string) uint32 {
	var h uint32
	for _, c := range id {
		h = h*31 + uint32(c)
	}
	return h
}

func (s *PremiumizeMonitorService) handleCompletedTransfer(ctx context.Context, transfer *premiumize.Transfer, media *models.Media) error {
	if media.OnDisk {
		return nil
	}

	link := s.fetchTransferLink(ctx, transfer)
	s.updateMediaAsAvailable(media, transfer, link)
	s.handleSeasonPackCompletion(ctx, media)

	if err := s.repo.SaveMedia(media); err != nil {
		return fmt.Errorf("updating media record: %w", err)
	}

	s.logTransferCompletion(media, transfer, link)
	return nil
}

func (s *PremiumizeMonitorService) fetchTransferLink(ctx context.Context, transfer *premiumize.Transfer) string {
	link, err := s.downloadManager.GetTransferLink(ctx, transfer)
	if err != nil {
		log.WithError(err).WithField("transfer_id", transfer.ID).Warn("Failed to get transfer link")
		return ""
	}
	return link
}

func (s *PremiumizeMonitorService) updateMediaAsAvailable(media *models.Media, transfer *premiumize.Transfer, link string) {
	media.File = link
	media.OnDisk = true
	media.TransferID = transfer.ID
	media.UpdatedAt = time.Now()
}

func (s *PremiumizeMonitorService) handleSeasonPackCompletion(ctx context.Context, media *models.Media) {
	if media.IsSeasonPack {
		if err := s.markSeasonEpisodesAvailable(ctx, media); err != nil {
			log.WithError(err).WithField("trakt", media.Trakt).Error("Failed to mark season episodes as available")
		}
	}
}

func (s *PremiumizeMonitorService) logTransferCompletion(media *models.Media, transfer *premiumize.Transfer, link string) {
	log.WithFields(log.Fields{
		"trakt":       media.Trakt,
		"title":       media.Title,
		"transfer_id": transfer.ID,
		"link":        link,
	}).Info("Successfully marked media as available in Premiumize")
}

func (s *PremiumizeMonitorService) handleFailedTransfer(ctx context.Context, transfer *premiumize.Transfer, media *models.Media) error {
	s.logTransferFailure(transfer, media)
	s.markNZBFailed(media.Trakt)
	
	if err := s.clearMediaTransferInfo(media); err != nil {
		return err
	}
	
	s.deleteFailedTransfer(ctx, transfer.ID)
	return nil
}

func (s *PremiumizeMonitorService) logTransferFailure(transfer *premiumize.Transfer, media *models.Media) {
	log.WithFields(log.Fields{
		"transfer_id": transfer.ID,
		"trakt":       media.Trakt,
		"title":       media.Title,
		"status":      transfer.Status,
	}).Error("Transfer failed")
}

func (s *PremiumizeMonitorService) clearMediaTransferInfo(media *models.Media) error {
	media.DownloadID = 0
	media.TransferID = ""
	media.UpdatedAt = time.Now()
	if err := s.repo.SaveMedia(media); err != nil {
		return fmt.Errorf("clearing transfer info: %w", err)
	}
	return nil
}

func (s *PremiumizeMonitorService) deleteFailedTransfer(ctx context.Context, transferID string) {
	if err := s.premiumizeClient.DeleteTransfer(ctx, transferID); err != nil {
		log.WithError(err).WithField("transfer_id", transferID).Warn("Failed to delete failed transfer")
	}
}

func (s *PremiumizeMonitorService) markSeasonEpisodesAvailable(ctx context.Context, seasonPackMedia *models.Media) error {
	if !s.isSeasonPackEligible(seasonPackMedia) {
		return nil
	}

	episodes, err := s.repo.GetEpisodesBySeason(seasonPackMedia.ShowIMDBID, seasonPackMedia.Season)
	if err != nil {
		return fmt.Errorf("getting episodes for season: %w", err)
	}

	s.updateEpisodes(episodes, seasonPackMedia)
	s.logSeasonPackCompletion(seasonPackMedia, len(episodes))
	return nil
}

func (s *PremiumizeMonitorService) isSeasonPackEligible(media *models.Media) bool {
	return media.IsSeasonPack && media.IsEpisode()
}

func (s *PremiumizeMonitorService) updateEpisodes(episodes []*models.Media, seasonPackMedia *models.Media) {
	for _, episode := range episodes {
		if !episode.OnDisk {
			s.markEpisodeAvailable(episode, seasonPackMedia)
		}
	}
}

func (s *PremiumizeMonitorService) markEpisodeAvailable(episode, seasonPackMedia *models.Media) {
	episode.OnDisk = true
	episode.File = fmt.Sprintf("Part of season pack: %s", seasonPackMedia.TransferID)
	episode.IsSeasonPack = true
	episode.SeasonPackID = seasonPackMedia.SeasonPackID
	episode.UpdatedAt = time.Now()
	
	if err := s.repo.SaveMedia(episode); err != nil {
		log.WithError(err).WithField("trakt", episode.Trakt).Error("Failed to update episode")
	}
}

func (s *PremiumizeMonitorService) logSeasonPackCompletion(media *models.Media, episodeCount int) {
	log.WithFields(log.Fields{
		"show":          media.ShowTitle,
		"season":        media.Season,
		"episode_count": episodeCount,
	}).Info("Marked all season episodes as available")
}

func (s *PremiumizeMonitorService) markNZBFailed(traktID int64) {
	log.WithField("trakt", traktID).Info("Marking NZB as failed")
}

func (s *PremiumizeMonitorService) RunPeriodically(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.MonitorTransfers(ctx); err != nil {
				log.WithError(err).Error("Failed to monitor transfers")
			}
		}
	}
}


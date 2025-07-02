package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/amaumene/momenarr/nzbget"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

// DownloadService handles download operations
type DownloadService struct {
	repo       repository.Repository
	nzbGet     *nzbget.NZBGet
	nzbService *NZBService
	httpClient *http.Client
	category   string
	dupeMode   string
}

// NewDownloadService creates a new DownloadService
func NewDownloadService(repo repository.Repository, nzbGet *nzbget.NZBGet, nzbService *NZBService) *DownloadService {
	return &DownloadService{
		repo:       repo,
		nzbGet:     nzbGet,
		nzbService: nzbService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  false,
			},
		},
		category: "momenarr",
		dupeMode: "score",
	}
}

// DownloadNotOnDisk downloads all media that is not on disk
func (s *DownloadService) DownloadNotOnDisk() error {
	medias, err := s.repo.FindMediaNotOnDisk()
	if err != nil {
		return fmt.Errorf("finding media not on disk: %w", err)
	}

	log.WithField("count", len(medias)).Info("Processing downloads for media not on disk")

	for _, media := range medias {
		if err := s.processMediaDownload(media); err != nil {
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
				"trakt": media.Trakt,
				"title": media.Title,
			}).Error("Failed to process media download")
			continue
		}
	}

	log.Info("Finished processing downloads")
	return nil
}

// processMediaDownload processes download for a single media item
func (s *DownloadService) processMediaDownload(media *models.Media) error {
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

	if err := s.CreateDownload(media.Trakt, nzb); err != nil {
		return fmt.Errorf("creating download: %w", err)
	}

	return nil
}

// processMediaDownloadWithContext processes download for a single media item with context
func (s *DownloadService) processMediaDownloadWithContext(ctx context.Context, media *models.Media) error {
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
		return fmt.Errorf("creating download: %w", err)
	}

	return nil
}

// CreateDownload creates a download in NZBGet
func (s *DownloadService) CreateDownload(traktID int64, nzb *models.NZB) error {
	// Check if already in queue
	if exists, err := s.isAlreadyInQueue(nzb.Title); err != nil {
		return fmt.Errorf("checking if NZB is already in queue: %w", err)
	} else if exists {
		log.WithFields(log.Fields{
			"trakt": traktID,
			"title": nzb.Title,
		}).Info("NZB already in queue, skipping")
		return nil
	}

	// Create NZBGet input
	input, err := s.createNZBGetInput(nzb, traktID)
	if err != nil {
		return fmt.Errorf("creating NZBGet input: %w", err)
	}

	// Add to NZBGet
	downloadID, err := s.nzbGet.Append(input)
	if err != nil || downloadID <= 0 {
		return fmt.Errorf("adding to NZBGet queue: %w", err)
	}

	// Update media with download ID
	if err := s.repo.UpdateMediaDownloadID(traktID, downloadID); err != nil {
		return fmt.Errorf("updating media with download ID: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt":       traktID,
		"title":       nzb.Title,
		"download_id": downloadID,
	}).Info("Download started successfully")

	return nil
}

// CreateDownloadWithContext creates a download in NZBGet with context support
func (s *DownloadService) CreateDownloadWithContext(ctx context.Context, traktID int64, nzb *models.NZB) error {
	// Check context before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Check if already in queue
	if exists, err := s.isAlreadyInQueue(nzb.Title); err != nil {
		return fmt.Errorf("checking if NZB is already in queue: %w", err)
	} else if exists {
		log.WithFields(log.Fields{
			"trakt": traktID,
			"title": nzb.Title,
		}).Info("NZB already in queue, skipping")
		return nil
	}

	// Create NZBGet input with context
	input, err := s.createNZBGetInputWithContext(ctx, nzb, traktID)
	if err != nil {
		return fmt.Errorf("creating NZBGet input: %w", err)
	}

	// Add to NZBGet
	downloadID, err := s.nzbGet.Append(input)
	if err != nil || downloadID <= 0 {
		return fmt.Errorf("adding to NZBGet queue: %w", err)
	}

	// Update media with download ID
	if err := s.repo.UpdateMediaDownloadID(traktID, downloadID); err != nil {
		return fmt.Errorf("updating media with download ID: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt":       traktID,
		"title":       nzb.Title,
		"download_id": downloadID,
	}).Info("Download started successfully")

	return nil
}

// isAlreadyInQueue checks if the NZB is already in the download queue
func (s *DownloadService) isAlreadyInQueue(title string) (bool, error) {
	queue, err := s.nzbGet.ListGroups()
	if err != nil {
		return false, fmt.Errorf("getting NZBGet queue: %w", err)
	}

	for _, item := range queue {
		if item.NZBName == title {
			return true, nil
		}
	}

	return false, nil
}

// createNZBGetInput creates the input for NZBGet
func (s *DownloadService) createNZBGetInput(nzb *models.NZB, traktID int64) (*nzbget.AppendInput, error) {
	// Download NZB content
	resp, err := s.httpClient.Get(nzb.Link)
	if err != nil {
		return nil, fmt.Errorf("downloading NZB file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download NZB file, status: %s", resp.Status)
	}

	// Limit the size of NZB files we'll download (50MB max)
	const maxNZBSize = 50 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxNZBSize)

	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("reading NZB file content: %w", err)
	}

	encodedContent := base64.StdEncoding.EncodeToString(content)

	return &nzbget.AppendInput{
		Filename: nzb.Title + ".nzb",
		Content:  encodedContent,
		Category: s.category,
		DupeMode: s.dupeMode,
		Parameters: []*nzbget.Parameter{
			{
				Name:  "Trakt",
				Value: strconv.FormatInt(traktID, 10),
			},
		},
	}, nil
}

// createNZBGetInputWithContext creates the input for NZBGet with context support
func (s *DownloadService) createNZBGetInputWithContext(ctx context.Context, nzb *models.NZB, traktID int64) (*nzbget.AppendInput, error) {
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nzb.Link, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Download NZB content
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading NZB file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download NZB file, status: %s", resp.Status)
	}

	// Limit the size of NZB files we'll download (50MB max)
	const maxNZBSize = 50 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxNZBSize)

	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("reading NZB file content: %w", err)
	}

	encodedContent := base64.StdEncoding.EncodeToString(content)

	return &nzbget.AppendInput{
		Filename: nzb.Title + ".nzb",
		Content:  encodedContent,
		Category: s.category,
		DupeMode: s.dupeMode,
		Parameters: []*nzbget.Parameter{
			{
				Name:  "Trakt",
				Value: strconv.FormatInt(traktID, 10),
			},
		},
	}, nil
}

// GetDownloadStatus gets the status of a download
func (s *DownloadService) GetDownloadStatus(downloadID int64) (string, error) {
	queue, err := s.nzbGet.ListGroups()
	if err != nil {
		return "", fmt.Errorf("getting NZBGet queue: %w", err)
	}

	for _, item := range queue {
		if int64(item.NZBID) == downloadID {
			return string(item.Status), nil
		}
	}

	// Check history if not in queue
	history, err := s.nzbGet.History(false)
	if err != nil {
		return "", fmt.Errorf("getting NZBGet history: %w", err)
	}

	for _, item := range history {
		if int64(item.NZBID) == downloadID {
			return string(item.Status), nil
		}
	}

	return "NOT_FOUND", nil
}

// CancelDownload cancels a download
func (s *DownloadService) CancelDownload(downloadID int64) error {
	success, err := s.nzbGet.EditQueue("GroupDelete", "", []int64{downloadID})
	if err != nil {
		return fmt.Errorf("canceling download: %w", err)
	}
	if !success {
		return fmt.Errorf("failed to cancel download %d", downloadID)
	}

	log.WithField("download_id", downloadID).Info("Download canceled")
	return nil
}

// RetryFailedDownload retries a failed download
func (s *DownloadService) RetryFailedDownload(traktID int64) error {
	// Mark current NZB as failed
	if err := s.nzbService.MarkNZBFailed(traktID); err != nil {
		log.WithError(err).WithField("trakt", traktID).Error("Failed to mark NZB as failed")
	}

	// Get media and try to download again
	media, err := s.repo.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("getting media for retry: %w", err)
	}

	return s.processMediaDownload(media)
}

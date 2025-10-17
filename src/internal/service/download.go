package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
	log "github.com/sirupsen/logrus"
)

const (
	nzbFileExtension = ".nzb"
	decimalBase      = 10
)

var (
	hiddenHistoryEnabled = false
	emptyDuplicateErr    = fmt.Errorf("download already queued")
	emptyHistoryErr      = fmt.Errorf("download already completed")
)

type DownloadService struct {
	cfg            *config.Config
	mediaRepo      domain.MediaRepository
	downloadClient domain.DownloadClient
	httpClient     *http.Client
}

func NewDownloadService(cfg *config.Config, mediaRepo domain.MediaRepository, downloadClient domain.DownloadClient) *DownloadService {
	return &DownloadService{
		cfg:            cfg,
		mediaRepo:      mediaRepo,
		downloadClient: downloadClient,
		httpClient:     &http.Client{Timeout: cfg.HTTPTimeout},
	}
}

func (s *DownloadService) CreateDownload(ctx context.Context, traktID int64, nzb *domain.NZB) error {
	if traktID <= 0 {
		return fmt.Errorf("invalid traktID: %d", traktID)
	}

	media, err := s.mediaRepo.Get(ctx, traktID)
	if err != nil {
		return fmt.Errorf("getting media: %w", err)
	}

	if err := s.preventDuplicate(ctx, media, nzb.Title); err != nil {
		return err
	}

	downloadID, err := s.appendToDownloader(ctx, traktID, nzb)
	if err != nil {
		return fmt.Errorf("append download: %w", err)
	}

	return s.updateMediaDownloadID(ctx, media, downloadID)
}

func (s *DownloadService) preventDuplicate(ctx context.Context, media *domain.Media, title string) error {
	queued, err := s.checkQueue(ctx, title)
	if err != nil {
		return err
	}
	if queued {
		s.logAlreadyInQueue(media.TraktID, title)
		return emptyDuplicateErr
	}

	history, err := s.checkHistory(ctx, media)
	if err != nil {
		return err
	}
	if history {
		s.logAlreadyInHistory(media.TraktID, title)
		return emptyHistoryErr
	}

	return nil
}

func (s *DownloadService) checkQueue(ctx context.Context, title string) (bool, error) {
	queue, err := s.downloadClient.ListGroups(ctx)
	if err != nil {
		return false, fmt.Errorf("listing queue: %w", err)
	}

	for _, item := range queue {
		if item.NZBName == title {
			return true, nil
		}
	}
	return false, nil
}

func (s *DownloadService) checkHistory(ctx context.Context, media *domain.Media) (bool, error) {
	history, err := s.downloadClient.History(ctx, hiddenHistoryEnabled)
	if err != nil {
		return false, fmt.Errorf("listing history: %w", err)
	}

	for _, item := range history {
		if item.NZBID == media.DownloadID {
			return true, nil
		}
	}
	return false, nil
}

func (s *DownloadService) appendToDownloader(ctx context.Context, traktID int64, nzb *domain.NZB) (int64, error) {
	content, err := s.downloadNZBFile(ctx, nzb.Link)
	if err != nil {
		return 0, fmt.Errorf("downloading nzb file: %w", err)
	}

	input := s.buildDownloadInput(traktID, nzb.Title, content)
	downloadID, err := s.downloadClient.Append(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("appending to downloader: %w", err)
	}
	if downloadID <= 0 {
		return 0, fmt.Errorf("invalid download id returned")
	}

	return downloadID, nil
}

func (s *DownloadService) downloadNZBFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	return content, nil
}

func (s *DownloadService) buildDownloadInput(traktID int64, title string, content []byte) *domain.DownloadInput {
	return &domain.DownloadInput{
		Filename: title + nzbFileExtension,
		Content:  base64.StdEncoding.EncodeToString(content),
		Category: s.cfg.NZBCategory,
		DupeMode: s.cfg.NZBDupeMode,
		Parameters: map[string]string{
			"Trakt": formatTraktID(traktID),
		},
	}
}

func (s *DownloadService) updateMediaDownloadID(ctx context.Context, media *domain.Media, downloadID int64) error {
	media.DownloadID = downloadID
	if err := s.mediaRepo.Update(ctx, media.TraktID, media); err != nil {
		return fmt.Errorf("updating media: %w", err)
	}

	s.logDownloadStarted(media.TraktID, media.Title, downloadID)
	return nil
}

func (s *DownloadService) logAlreadyInQueue(traktID int64, title string) {
	log.WithFields(log.Fields{
		"traktID": traktID,
		"title":   title,
	}).Info("media already in download queue, skipping")
}

func (s *DownloadService) logAlreadyInHistory(traktID int64, title string) {
	log.WithFields(log.Fields{
		"traktID": traktID,
		"title":   title,
	}).Info("media already in download history, skipping")
}

func (s *DownloadService) logDownloadStarted(traktID int64, title string, downloadID int64) {
	log.WithFields(log.Fields{
		"traktID":    traktID,
		"title":      title,
		"downloadID": downloadID,
	}).Info("download added to nzbget queue")
}

func formatTraktID(traktID int64) string {
	return strconv.FormatInt(traktID, decimalBase)
}

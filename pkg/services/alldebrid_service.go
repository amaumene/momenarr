package services

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

type AllDebridService struct {
	client *AllDebridClient
	repo   repository.Repository
	apiKey string
}

func NewAllDebridService(repo repository.Repository, apiKey string) *AllDebridService {
	client := NewAllDebridClient(apiKey)

	return &AllDebridService{
		client: client,
		repo:   repo,
		apiKey: apiKey,
	}
}

// IsTorrentCached checks if a torrent is cached on AllDebrid using the hash
func (s *AllDebridService) IsTorrentCached(hash string) (bool, int64, error) {
	magnetURL := fmt.Sprintf("magnet:?xt=urn:btih:%s", hash)

	uploadResult, err := s.uploadMagnetForCheck(magnetURL)
	if err != nil {
		return false, 0, err
	}

	magnet, err := s.validateUploadResult(uploadResult)
	if err != nil {
		return false, 0, err
	}

	return s.handleCacheCheck(magnet)
}

// uploadMagnetForCheck uploads magnet to check if cached
func (s *AllDebridService) uploadMagnetForCheck(magnetURL string) (*MagnetUploadResponse, error) {
	uploadResult, err := s.client.UploadMagnet([]string{magnetURL})
	if err != nil {
		return nil, fmt.Errorf("failed to upload magnet: %w", err)
	}
	return uploadResult, nil
}

// validateUploadResult validates the upload response
func (s *AllDebridService) validateUploadResult(result *MagnetUploadResponse) (*struct {
	ID    int    `json:"id"`
	Hash  string `json:"hash"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Ready bool   `json:"ready"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}, error) {
	if result.Error != nil {
		return nil, fmt.Errorf("upload error: %s", result.Error.Message)
	}

	if len(result.Data.Magnets) == 0 {
		return nil, nil
	}

	magnet := &result.Data.Magnets[0]
	if magnet.Error != nil {
		return nil, fmt.Errorf("magnet error: %s", magnet.Error.Message)
	}

	return magnet, nil
}

// handleCacheCheck handles the cache check result
func (s *AllDebridService) handleCacheCheck(magnet *struct {
	ID    int    `json:"id"`
	Hash  string `json:"hash"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Ready bool   `json:"ready"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}) (bool, int64, error) {
	if magnet == nil {
		return false, 0, nil
	}

	if magnet.Ready {
		return true, int64(magnet.ID), nil
	}

	// If not ready, delete it and return false
	if _, delErr := s.client.DeleteMagnet(magnet.ID); delErr != nil {
		log.WithError(delErr).Error("Failed to delete non-cached magnet")
	}

	return false, 0, nil
}

// UploadTorrent uploads a torrent to AllDebrid and returns the AllDebrid ID
func (s *AllDebridService) UploadTorrent(result *models.TorrentSearchResult) (int64, error) {
	magnetURL := s.buildMagnetURL(result)
	s.logTorrentUpload(result, magnetURL)

	uploadResult, err := s.uploadMagnet(magnetURL)
	if err != nil {
		return 0, err
	}

	magnet, err := s.processMagnetUpload(uploadResult)
	if err != nil {
		return 0, err
	}

	s.logSuccessfulUpload(result, magnet.ID)
	return int64(magnet.ID), nil
}

// buildMagnetURL builds magnet URL from torrent result
func (s *AllDebridService) buildMagnetURL(result *models.TorrentSearchResult) string {
	if result.MagnetURL != "" {
		return result.MagnetURL
	}
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", result.Hash, url.QueryEscape(result.Title))
}

// logTorrentUpload logs torrent upload details
func (s *AllDebridService) logTorrentUpload(result *models.TorrentSearchResult, magnetURL string) {
	log.WithFields(log.Fields{
		"hash":       result.Hash,
		"title":      result.Title,
		"magnet_url": magnetURL,
	}).Debug("Uploading torrent to AllDebrid")
}

// uploadMagnet uploads magnet URL to AllDebrid
func (s *AllDebridService) uploadMagnet(magnetURL string) (*MagnetUploadResponse, error) {
	uploadResult, err := s.client.UploadMagnet([]string{magnetURL})
	if err != nil {
		return nil, fmt.Errorf("failed to upload magnet to AllDebrid: %w", err)
	}
	return uploadResult, nil
}

// processMagnetUpload processes the magnet upload response
func (s *AllDebridService) processMagnetUpload(uploadResult *MagnetUploadResponse) (*struct {
	ID    int    `json:"id"`
	Hash  string `json:"hash"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Ready bool   `json:"ready"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}, error) {
	if uploadResult.Error != nil {
		return nil, fmt.Errorf("upload error: %s", uploadResult.Error.Message)
	}

	if len(uploadResult.Data.Magnets) == 0 {
		return nil, fmt.Errorf("no magnet data returned from AllDebrid")
	}

	magnet := &uploadResult.Data.Magnets[0]
	if magnet.Error != nil {
		return nil, fmt.Errorf("magnet upload error: %s", magnet.Error.Message)
	}

	return magnet, nil
}

// logSuccessfulUpload logs successful torrent upload
func (s *AllDebridService) logSuccessfulUpload(result *models.TorrentSearchResult, magnetID int) {
	log.WithFields(log.Fields{
		"hash":         result.Hash,
		"title":        result.Title,
		"alldebrid_id": magnetID,
	}).Info("Successfully uploaded torrent to AllDebrid")
}

// WaitForTorrentReady waits for a torrent to be ready in AllDebrid
func (s *AllDebridService) WaitForTorrentReady(allDebridID int64, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	return s.pollTorrentStatus(ctx, ticker, allDebridID)
}

// pollTorrentStatus polls torrent status until ready or timeout
func (s *AllDebridService) pollTorrentStatus(ctx context.Context, ticker *time.Ticker, allDebridID int64) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for torrent to be ready")
		case <-ticker.C:
			if ready, err := s.checkTorrentReady(allDebridID); err != nil {
				log.WithError(err).Debug("Failed to get magnet status, retrying...")
				continue
			} else if ready {
				return nil
			}
		}
	}
}

// checkTorrentReady checks if torrent is ready
func (s *AllDebridService) checkTorrentReady(allDebridID int64) (bool, error) {
	result, err := s.client.GetMagnetStatus(int(allDebridID))
	if err != nil {
		return false, err
	}

	magnetStatus := result.Data.Magnet
	s.logTorrentStatus(magnetStatus.Status, magnetStatus.StatusCode, magnetStatus.Ready)

	return magnetStatus.Ready, nil
}

// logTorrentStatus logs torrent status
func (s *AllDebridService) logTorrentStatus(status string, statusCode int, ready bool) {
	fields := log.Fields{
		"status":      status,
		"status_code": statusCode,
	}

	if ready {
		log.WithFields(fields).Debug("Torrent is ready")
	} else {
		log.WithFields(fields).Debug("Torrent not ready yet, waiting...")
	}
}

// DownloadFile downloads a file from AllDebrid
func (s *AllDebridService) DownloadFile(allDebridID int64, torrentResult *models.TorrentSearchResult, media *models.Media) error {
	magnetFiles, err := s.getMagnetFiles(allDebridID)
	if err != nil {
		return err
	}

	if torrentResult.IsSeasonPack() {
		return s.handleSeasonPack(torrentResult, magnetFiles, media)
	}

	return s.downloadBestVideoFile(magnetFiles, media)
}

// getMagnetFiles retrieves files for a magnet
func (s *AllDebridService) getMagnetFiles(allDebridID int64) (*struct {
	ID    int `json:"id"`
	Files []struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	} `json:"files"`
}, error) {
	result, err := s.client.GetMagnetFiles(int(allDebridID))
	if err != nil {
		return nil, fmt.Errorf("failed to get magnet files: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("get files error: %s", result.Error.Message)
	}

	if len(result.Data.Magnets) == 0 {
		return nil, fmt.Errorf("no files found for magnet")
	}

	return &result.Data.Magnets[0], nil
}

// downloadBestVideoFile finds and downloads the best video file
func (s *AllDebridService) downloadBestVideoFile(magnetFiles *struct {
	ID    int `json:"id"`
	Files []struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	} `json:"files"`
}, media *models.Media) error {
	bestFile := s.findBestVideoFile(magnetFiles)
	if bestFile == nil {
		return fmt.Errorf("no video file found in torrent")
	}

	return s.downloadSingleFile(bestFile.Link, bestFile.Name, media)
}

// findBestVideoFile finds the largest video file
func (s *AllDebridService) findBestVideoFile(magnetFiles *struct {
	ID    int `json:"id"`
	Files []struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	} `json:"files"`
}) *struct {
	Name string `json:"n"`
	Size int64  `json:"s"`
	Link string `json:"l"`
} {
	var bestFile *struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	}

	for i := range magnetFiles.Files {
		file := &magnetFiles.Files[i]
		if isVideoFile(file.Name) {
			if bestFile == nil || file.Size > bestFile.Size {
				bestFile = file
			}
		}
	}

	return bestFile
}

// handleSeasonPack handles downloading files from a season pack
func (s *AllDebridService) handleSeasonPack(torrentResult *models.TorrentSearchResult, magnet *struct {
	ID    int `json:"id"`
	Files []struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	} `json:"files"`
}, media *models.Media) error {
	episodeFiles := s.extractEpisodeFiles(torrentResult, magnet)
	s.logSeasonPackInfo(torrentResult, episodeFiles)

	return s.downloadRequestedEpisode(episodeFiles, media)
}

// extractEpisodeFiles extracts episode files from season pack
func (s *AllDebridService) extractEpisodeFiles(torrentResult *models.TorrentSearchResult, magnet *struct {
	ID    int `json:"id"`
	Files []struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	} `json:"files"`
}) map[int]*struct {
	Name string `json:"n"`
	Size int64  `json:"s"`
	Link string `json:"l"`
} {
	episodeFiles := make(map[int]*struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	})

	episodeRegex := regexp.MustCompile(`(?i)s(\d{2})e(\d{2})`)
	season := torrentResult.ExtractSeason()

	for i := range magnet.Files {
		file := &magnet.Files[i]
		if episode := s.extractEpisodeFromFile(file, episodeRegex, season); episode > 0 {
			episodeFiles[episode] = file
		}
	}

	return episodeFiles
}

// extractEpisodeFromFile extracts episode number from file
func (s *AllDebridService) extractEpisodeFromFile(file *struct {
	Name string `json:"n"`
	Size int64  `json:"s"`
	Link string `json:"l"`
}, regex *regexp.Regexp, targetSeason int) int {
	if !isVideoFile(file.Name) {
		return 0
	}

	matches := regex.FindStringSubmatch(file.Name)
	if len(matches) != 3 {
		return 0
	}

	fileSeason, _ := strconv.Atoi(matches[1])
	episode, _ := strconv.Atoi(matches[2])

	if fileSeason == targetSeason {
		return episode
	}
	return 0
}

// logSeasonPackInfo logs season pack information
func (s *AllDebridService) logSeasonPackInfo(torrentResult *models.TorrentSearchResult,
	episodeFiles map[int]*struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	}) {
	var episodesInPack []int
	for ep := range episodeFiles {
		episodesInPack = append(episodesInPack, ep)
	}

	log.WithFields(log.Fields{
		"torrent_title":    torrentResult.Title,
		"season":           torrentResult.ExtractSeason(),
		"episodes_in_pack": episodesInPack,
	}).Info("Found episodes in season pack")
}

// downloadRequestedEpisode downloads the requested episode from pack
func (s *AllDebridService) downloadRequestedEpisode(episodeFiles map[int]*struct {
	Name string `json:"n"`
	Size int64  `json:"s"`
	Link string `json:"l"`
}, media *models.Media) error {
	if file, ok := episodeFiles[int(media.Number)]; ok {
		return s.downloadSingleFile(file.Link, file.Name, media)
	}
	return fmt.Errorf("episode %d not found in season pack", media.Number)
}

// downloadSingleFile marks a file as available via AllDebrid (generates direct link)
func (s *AllDebridService) downloadSingleFile(link, filename string, media *models.Media) error {
	// Unlock the link to get direct download URL
	result, err := s.client.UnlockLink(link)
	if err != nil {
		return fmt.Errorf("failed to unlock link: %w", err)
	}

	if result.Error != nil {
		return fmt.Errorf("unlock error: %s", result.Error.Message)
	}

	if result.Data.Delayed != 0 {
		return fmt.Errorf("link generation delayed, try later")
	}

	// Store the AllDebrid direct link and mark as available
	media.File = result.Data.Link // Store the direct AllDebrid link
	media.OnDisk = true           // Mark as "available" via AllDebrid
	if err := s.repo.SaveMedia(media); err != nil {
		return fmt.Errorf("failed to update media: %w", err)
	}

	log.WithFields(log.Fields{
		"link":     result.Data.Link,
		"filename": filename,
		"trakt_id": media.Trakt,
		"title":    media.Title,
	}).Info("Successfully linked file from AllDebrid")

	return nil
}

// DeleteMagnet deletes a magnet from AllDebrid
func (s *AllDebridService) DeleteMagnet(allDebridID int64) error {
	_, err := s.client.DeleteMagnet(int(allDebridID))
	if err != nil {
		return fmt.Errorf("failed to delete magnet: %w", err)
	}

	log.WithField("alldebrid_id", allDebridID).Info("Successfully deleted magnet from AllDebrid")
	return nil
}

// GetMagnetStatus gets the status of a magnet by ID
func (s *AllDebridService) GetMagnetStatus(allDebridID int64) (string, error) {
	result, err := s.client.GetMagnetStatus(int(allDebridID))
	if err != nil {
		return "ERROR", err
	}

	return result.Data.Magnet.Status, nil
}

// isVideoFile checks if a filename is a video file
func isVideoFile(filename string) bool {
	videoExtensions := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg"}

	lowercaseFilename := strings.ToLower(filename)
	for _, ext := range videoExtensions {
		if strings.HasSuffix(lowercaseFilename, ext) {
			return true
		}
	}

	return false
}

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

	// Upload magnet to check if it's cached (AllDebrid will instantly download if cached)
	uploadResult, err := s.client.UploadMagnet([]string{magnetURL})
	if err != nil {
		return false, 0, fmt.Errorf("failed to upload magnet: %w", err)
	}

	if uploadResult.Error != nil {
		return false, 0, fmt.Errorf("upload error: %s", uploadResult.Error.Message)
	}

	if len(uploadResult.Data.Magnets) == 0 {
		return false, 0, nil
	}

	magnet := uploadResult.Data.Magnets[0]
	if magnet.Error != nil {
		return false, 0, fmt.Errorf("magnet error: %s", magnet.Error.Message)
	}

	// If Ready is true, it was cached
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
	magnetURL := result.MagnetURL
	if magnetURL == "" {
		magnetURL = fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", result.Hash, url.QueryEscape(result.Title))
	}

	log.WithFields(log.Fields{
		"hash":       result.Hash,
		"title":      result.Title,
		"magnet_url": magnetURL,
	}).Debug("Uploading torrent to AllDebrid")

	uploadResult, err := s.client.UploadMagnet([]string{magnetURL})
	if err != nil {
		return 0, fmt.Errorf("failed to upload magnet to AllDebrid: %w", err)
	}

	if uploadResult.Error != nil {
		return 0, fmt.Errorf("upload error: %s", uploadResult.Error.Message)
	}

	if len(uploadResult.Data.Magnets) == 0 {
		return 0, fmt.Errorf("no magnet data returned from AllDebrid")
	}

	magnet := uploadResult.Data.Magnets[0]
	if magnet.Error != nil {
		return 0, fmt.Errorf("magnet upload error: %s", magnet.Error.Message)
	}

	log.WithFields(log.Fields{
		"hash":         result.Hash,
		"title":        result.Title,
		"alldebrid_id": magnet.ID,
	}).Info("Successfully uploaded torrent to AllDebrid")

	return int64(magnet.ID), nil
}

// WaitForTorrentReady waits for a torrent to be ready in AllDebrid
func (s *AllDebridService) WaitForTorrentReady(allDebridID int64, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for torrent to be ready")
		case <-ticker.C:
			result, err := s.client.GetMagnetStatus(int(allDebridID))
			if err != nil {
				log.WithError(err).Debug("Failed to get magnet status, retrying...")
				continue
			}

			if result.Data.Magnet.Ready {
				log.WithFields(log.Fields{
					"status":      result.Data.Magnet.Status,
					"status_code": result.Data.Magnet.StatusCode,
				}).Debug("Torrent is ready")
				return nil
			}

			log.WithFields(log.Fields{
				"status":      result.Data.Magnet.Status,
				"status_code": result.Data.Magnet.StatusCode,
			}).Debug("Torrent not ready yet, waiting...")
		}
	}
}

// DownloadFile downloads a file from AllDebrid
func (s *AllDebridService) DownloadFile(allDebridID int64, torrentResult *models.TorrentSearchResult, media *models.Media) error {
	// Get magnet files
	result, err := s.client.GetMagnetFiles(int(allDebridID))
	if err != nil {
		return fmt.Errorf("failed to get magnet files: %w", err)
	}

	if result.Error != nil {
		return fmt.Errorf("get files error: %s", result.Error.Message)
	}

	if len(result.Data.Magnets) == 0 {
		return fmt.Errorf("no files found for magnet")
	}

	magnetFiles := result.Data.Magnets[0]

	// Handle season packs
	if torrentResult.IsSeasonPack() {
		return s.handleSeasonPack(torrentResult, &magnetFiles, media)
	}

	// For single episodes or movies, find the best video file
	var bestFile *struct{
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

	if bestFile == nil {
		return fmt.Errorf("no video file found in torrent")
	}

	// Generate download link and mark as available
	return s.downloadSingleFile(bestFile.Link, bestFile.Name, media)
}

// handleSeasonPack handles downloading files from a season pack
func (s *AllDebridService) handleSeasonPack(torrentResult *models.TorrentSearchResult, magnet *struct{
	ID    int `json:"id"`
	Files []struct {
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	} `json:"files"`
}, media *models.Media) error {
	// Extract episode numbers from files
	episodeFiles := make(map[int]*struct{
		Name string `json:"n"`
		Size int64  `json:"s"`
		Link string `json:"l"`
	})

	episodeRegex := regexp.MustCompile(`(?i)s(\d{2})e(\d{2})`)
	season := torrentResult.ExtractSeason()

	for i := range magnet.Files {
		file := &magnet.Files[i]
		if !isVideoFile(file.Name) {
			continue
		}

		matches := episodeRegex.FindStringSubmatch(file.Name)
		if len(matches) == 3 {
			fileSeason, _ := strconv.Atoi(matches[1])
			episode, _ := strconv.Atoi(matches[2])

			if fileSeason == season {
				episodeFiles[episode] = file
			}
		}
	}

	// Log episodes found in pack
	var episodesInPack []int
	for ep := range episodeFiles {
		episodesInPack = append(episodesInPack, ep)
	}

	log.WithFields(log.Fields{
		"torrent_title":    torrentResult.Title,
		"season":          season,
		"episodes_in_pack": episodesInPack,
	}).Info("Found episodes in season pack")

	// Download only the requested episode
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


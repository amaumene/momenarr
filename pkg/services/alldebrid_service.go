package services

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amaumene/momenarr/pkg/alldebrid"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)

type AllDebridService struct {
	client     *alldebrid.Client
	repo       repository.Repository
	apiKey     string
	httpClient *http.Client
}

func NewAllDebridService(repo repository.Repository, apiKey string) *AllDebridService {
	return &AllDebridService{
		client: alldebrid.NewClient(),
		repo:   repo,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// IsTorrentCached checks if a torrent is cached on AllDebrid
func (s *AllDebridService) IsTorrentCached(hash string) (bool, int64, error) {
	magnetURL := fmt.Sprintf("magnet:?xt=urn:btih:%s", hash)

	uploadResp, err := s.client.UploadMagnet(s.apiKey, []string{magnetURL})
	if err != nil {
		return false, 0, fmt.Errorf("failed to upload magnet: %w", err)
	}

	if uploadResp.Status != "success" {
		if uploadResp.Error != nil {
			return false, 0, fmt.Errorf("AllDebrid error: %s - %s", uploadResp.Error.Code, uploadResp.Error.Message)
		}
		return false, 0, fmt.Errorf("AllDebrid error: %s", uploadResp.Status)
	}

	if len(uploadResp.Data.Magnets) == 0 {
		return false, 0, nil
	}

	magnet := uploadResp.Data.Magnets[0]
	if magnet.Error != nil {
		return false, 0, fmt.Errorf("magnet error: %s - %s", magnet.Error.Code, magnet.Error.Message)
	}

	time.Sleep(2 * time.Second)

	status, err := s.client.GetMagnetStatus(s.apiKey, []int64{magnet.ID})
	if err != nil {
		return false, 0, fmt.Errorf("failed to get magnet status: %w", err)
	}

	if status.Status != "success" || len(status.Data.Magnets) == 0 {
		return false, 0, fmt.Errorf("no magnet status returned")
	}

	statusMagnet := status.Data.Magnets[0]
	isCached := statusMagnet.StatusCode == 4

	if !isCached {
		if _, delErr := s.client.DeleteMagnet(s.apiKey, magnet.ID); delErr != nil {
			log.WithError(delErr).Error("Failed to delete non-cached magnet")
		}
		return false, 0, nil
	}

	return true, magnet.ID, nil
}

// UploadTorrent uploads a torrent to AllDebrid
func (s *AllDebridService) UploadTorrent(torrent *models.Torrent) error {
	// Create magnet URL
	magnetURL := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", torrent.Hash, url.QueryEscape(torrent.Title))

	log.WithFields(log.Fields{
		"hash":       torrent.Hash,
		"title":      torrent.Title,
		"magnet_url": magnetURL,
	}).Debug("Uploading torrent to AllDebrid")

	// Upload to AllDebrid
	resp, err := s.client.UploadMagnet(s.apiKey, []string{magnetURL})
	if err != nil {
		return fmt.Errorf("failed to upload magnet: %w", err)
	}

	// Check response
	if resp.Status != "success" {
		if resp.Error != nil {
			return fmt.Errorf("AllDebrid error: %s - %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("AllDebrid error: %s", resp.Status)
	}

	// Get the uploaded magnet info
	if len(resp.Data.Magnets) == 0 {
		return fmt.Errorf("no magnet data returned from AllDebrid")
	}

	magnet := resp.Data.Magnets[0]
	if magnet.Error != nil {
		return fmt.Errorf("magnet upload error: %s - %s", magnet.Error.Code, magnet.Error.Message)
	}

	// Update torrent with AllDebrid ID
	torrent.AllDebridID = magnet.ID
	if err := s.repo.SaveTorrent(torrent); err != nil {
		return fmt.Errorf("failed to save torrent with AllDebrid ID: %w", err)
	}

	log.WithFields(log.Fields{
		"hash":         torrent.Hash,
		"title":        torrent.Title,
		"alldebrid_id": magnet.ID,
	}).Info("Successfully uploaded torrent to AllDebrid")

	return nil
}

// WaitForTorrentReady waits for a torrent to be ready in AllDebrid
func (s *AllDebridService) WaitForTorrentReady(torrent *models.Torrent, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for torrent to be ready")
		case <-ticker.C:
			status, err := s.client.GetMagnetStatus(s.apiKey, []int64{torrent.AllDebridID})
			if err != nil {
				log.WithError(err).Debug("Failed to get magnet status, retrying...")
				continue
			}

			if status.Status != "success" || len(status.Data.Magnets) == 0 {
				continue
			}

			magnet := status.Data.Magnets[0]
			if magnet.StatusCode == 4 {
				log.WithFields(log.Fields{
					"status":      magnet.Status,
					"status_code": magnet.StatusCode,
					"ready":       magnet.Ready,
				}).Debug("Torrent is ready (status code 4)")
				return nil
			}

			log.WithFields(log.Fields{
				"status":      magnet.Status,
				"status_code": magnet.StatusCode,
				"ready":       magnet.Ready,
			}).Debug("Torrent not ready yet, waiting...")
		}
	}
}

// DownloadFile downloads a file from AllDebrid
func (s *AllDebridService) DownloadFile(torrent *models.Torrent, media *models.Media) error {
	// Get magnet files
	filesResp, err := s.client.GetMagnetFiles(s.apiKey, strconv.FormatInt(torrent.AllDebridID, 10))
	if err != nil {
		return fmt.Errorf("failed to get magnet files: %w", err)
	}

	if filesResp.Status != "success" || len(filesResp.Data.Magnets) == 0 {
		return fmt.Errorf("no files found for magnet")
	}

	magnet := filesResp.Data.Magnets[0]

	// Handle season packs
	if torrent.IsSeasonPack {
		// Convert magnet to expected structure
		// Convert links to expected type
		var convertedLinks []struct {
			Link     string
			Filename string
			Size     int64
		}
		for _, link := range magnet.Links {
			convertedLinks = append(convertedLinks, struct {
				Link     string
				Filename string
				Size     int64
			}{
				Link:     link.Link,
				Filename: link.Filename,
				Size:     link.Size,
			})
		}

		convertedMagnet := struct {
			ID    int64
			Hash  string
			Name  string
			Size  int64
			Ready bool
			Links []struct {
				Link     string
				Filename string
				Size     int64
			}
		}{
			ID:    magnet.ID,
			Hash:  magnet.Hash,
			Name:  magnet.Name,
			Size:  magnet.Size,
			Ready: magnet.Ready,
			Links: convertedLinks,
		}
		return s.handleSeasonPack(torrent, convertedMagnet, media)
	}

	// For single episodes or movies, find the best video file
	var bestFile *struct {
		Link     string
		Filename string
		Size     int64
	}

	for i := range magnet.Links {
		file := &magnet.Links[i]
		if isVideoFile(file.Filename) {
			if bestFile == nil || file.Size > bestFile.Size {
				bestFile = &struct {
					Link     string
					Filename string
					Size     int64
				}{
					Link:     file.Link,
					Filename: file.Filename,
					Size:     file.Size,
				}
			}
		}
	}

	if bestFile == nil {
		return fmt.Errorf("no video file found in torrent")
	}

	// Download the file
	return s.downloadSingleFile(bestFile.Link, bestFile.Filename, media)
}

// handleSeasonPack handles downloading files from a season pack
func (s *AllDebridService) handleSeasonPack(torrent *models.Torrent, magnet struct {
	ID    int64
	Hash  string
	Name  string
	Size  int64
	Ready bool
	Links []struct {
		Link     string
		Filename string
		Size     int64
	}
}, media *models.Media) error {
	// Extract episode numbers from files
	episodeFiles := make(map[int]struct {
		Link     string
		Filename string
		Size     int64
	})

	episodeRegex := regexp.MustCompile(`(?i)s(\d{2})e(\d{2})`)

	for _, file := range magnet.Links {
		if !isVideoFile(file.Filename) {
			continue
		}

		matches := episodeRegex.FindStringSubmatch(file.Filename)
		if len(matches) == 3 {
			season, _ := strconv.Atoi(matches[1])
			episode, _ := strconv.Atoi(matches[2])

			if season == torrent.Season {
				episodeFiles[episode] = struct {
					Link     string
					Filename string
					Size     int64
				}{
					Link:     file.Link,
					Filename: file.Filename,
					Size:     file.Size,
				}
			}
		}
	}

	// Update torrent with episodes in pack
	var episodesInPack []int
	for ep := range episodeFiles {
		episodesInPack = append(episodesInPack, ep)
	}

	if err := s.repo.UpdateTorrentSeasonPack(torrent.ID, episodesInPack); err != nil {
		log.WithError(err).Error("Failed to update torrent season pack info")
	}

	// Download only the requested episode
	if file, ok := episodeFiles[int(media.Number)]; ok {
		return s.downloadSingleFile(file.Link, file.Filename, media)
	}

	return fmt.Errorf("episode %d not found in season pack", media.Number)
}

// downloadSingleFile marks a file as available via AllDebrid (no local download)
func (s *AllDebridService) downloadSingleFile(link, filename string, media *models.Media) error {
	// Unlock the link to verify it's accessible
	unlockResp, err := s.client.UnlockLink(s.apiKey, link)
	if err != nil {
		return fmt.Errorf("failed to unlock link: %w", err)
	}

	if unlockResp.Status != "success" {
		return fmt.Errorf("failed to unlock link: %s", unlockResp.Status)
	}

	// Store the AllDebrid direct link and mark as available
	media.File = unlockResp.Data.Link // Store the direct AllDebrid link
	media.OnDisk = true               // Mark as "available" via AllDebrid
	if err := s.repo.SaveMedia(media); err != nil {
		return fmt.Errorf("failed to update media: %w", err)
	}

	log.WithFields(log.Fields{
		"link":     unlockResp.Data.Link,
		"filename": filename,
		"trakt_id": media.Trakt,
		"title":    media.Title,
	}).Info("Successfully linked file from AllDebrid")

	return nil
}

// DeleteMagnet deletes a magnet from AllDebrid
func (s *AllDebridService) DeleteMagnet(allDebridID int64) error {
	resp, err := s.client.DeleteMagnet(s.apiKey, allDebridID)
	if err != nil {
		return fmt.Errorf("failed to delete magnet: %w", err)
	}

	if resp.Status != "success" {
		if resp.Error != nil {
			return fmt.Errorf("failed to delete magnet: %s - %s", resp.Error.Code, resp.Error.Message)
		}
		return fmt.Errorf("failed to delete magnet: %s", resp.Status)
	}

	log.WithField("alldebrid_id", allDebridID).Info("Successfully deleted magnet from AllDebrid")
	return nil
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

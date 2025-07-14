package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/utils"
	log "github.com/sirupsen/logrus"
)

// TorrentService handles torrent search and management operations
type TorrentService struct {
	repo           repository.Repository
	searchService  *TorrentSearchService
	blacklistFile  string
	blacklistCache map[string]struct{}
	blacklistMu    sync.RWMutex
}

// NewTorrentService creates a new TorrentService
func NewTorrentService(repo repository.Repository, blacklistFile string) *TorrentService {
	return &TorrentService{
		repo:           repo,
		searchService:  NewTorrentSearchService(),
		blacklistFile:  blacklistFile,
		blacklistCache: make(map[string]struct{}),
	}
}

// NewTorrentServiceWithTrakt creates a new TorrentService with Trakt support
func NewTorrentServiceWithTrakt(repo repository.Repository, blacklistFile string, traktService *TraktService) *TorrentService {
	return &TorrentService{
		repo:           repo,
		searchService:  NewTorrentSearchServiceWithTrakt(traktService),
		blacklistFile:  blacklistFile,
		blacklistCache: make(map[string]struct{}),
	}
}

// GetBestTorrent retrieves the best torrent for a given Trakt ID
func (s *TorrentService) GetBestTorrent(traktID int64) (*models.Torrent, error) {
	return s.repo.GetBestTorrent(traktID)
}

// GetSortedTorrents retrieves all torrents for a given Trakt ID sorted by preference
func (s *TorrentService) GetSortedTorrents(traktID int64) ([]*models.Torrent, error) {
	torrents, err := s.repo.FindAllTorrentsByTraktID(traktID)
	if err != nil {
		return nil, fmt.Errorf("finding torrents: %w", err)
	}

	// Filter out failed torrents
	var validTorrents []*models.Torrent
	for _, torrent := range torrents {
		if !torrent.Failed {
			validTorrents = append(validTorrents, torrent)
		}
	}

	if len(validTorrents) == 0 {
		return nil, fmt.Errorf("no valid torrents found for Trakt ID %d", traktID)
	}

	// Sort by preference (size descending for now)
	for i := 0; i < len(validTorrents); i++ {
		for j := i + 1; j < len(validTorrents); j++ {
			if validTorrents[j].Size > validTorrents[i].Size {
				validTorrents[i], validTorrents[j] = validTorrents[j], validTorrents[i]
			}
		}
	}

	return validTorrents, nil
}

// PopulateTorrents is deprecated - using real-time search
func (s *TorrentService) PopulateTorrents() error {
	return nil
}

// PopulateTorrentsWithContext is deprecated - using real-time search
func (s *TorrentService) PopulateTorrentsWithContext(ctx context.Context) error {
	return nil
}

// FindBestCachedTorrent searches for torrents in real-time and returns the best one cached on AllDebrid
func (s *TorrentService) FindBestCachedTorrent(media *models.Media, allDebridService *AllDebridService) (*models.TorrentSearchResult, error) {
	// Determine media type for search
	mediaType := "movie"
	if media.IsEpisode() {
		mediaType = "series"
	}

	// Search for torrents with year validation for movies
	var results []models.TorrentSearchResult
	var err error

	log.WithFields(log.Fields{
		"trakt_id":   media.Trakt,
		"title":      media.Title,
		"media_type": mediaType,
		"season":     media.Season,
		"number":     media.Number,
		"year":       media.Year,
		"imdb":       media.IMDB,
		"trakt_slug": media.TraktSlug,
	}).Info("Starting torrent search")

	if media.IsMovie() && media.Year > 0 {
		// For movies, use year-aware search with Trakt slug
		results, err = s.searchService.SearchWithYearAndTraktSlug(
			media.IMDB,
			media.Title,
			mediaType,
			int(media.Season),
			int(media.Number),
			int(media.Year),
			media.TraktSlug,
		)
	} else {
		// For TV shows or movies without year, use regular search with Trakt slug
		results, err = s.searchService.SearchWithYearAndTraktSlug(
			media.IMDB,
			media.Title,
			mediaType,
			int(media.Season),
			int(media.Number),
			0,
			media.TraktSlug,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("searching torrents for media %d: %w", media.Trakt, err)
	}

	if len(results) == 0 {
		log.WithField("trakt_id", media.Trakt).Info("No torrents found")
		return nil, nil
	}

	// Filter out blacklisted torrents
	filteredResults, err := s.filterBlacklistedTorrents(results)
	if err != nil {
		return nil, fmt.Errorf("filtering blacklisted torrents: %w", err)
	}

	if len(filteredResults) == 0 {
		log.WithField("trakt_id", media.Trakt).Info("All torrents blacklisted")
		return nil, nil
	}

	s.sortTorrentResults(filteredResults)

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"count":    len(filteredResults),
	}).Info("Checking AllDebrid cache")

	// Check each torrent in order of overall quality (best from all providers)
	for i, result := range filteredResults {
		if result.Hash == "" {
			continue
		}

		// Check if cached on AllDebrid
		cached, _, err := allDebridService.IsTorrentCached(result.Hash)
		if err != nil {
			log.WithError(err).WithField("hash", result.Hash).Error("Failed to check AllDebrid cache")
			continue
		}

		if cached {
			log.WithFields(log.Fields{
				"trakt_id": media.Trakt,
				"title":    result.Title,
				"source":   result.Source,
				"rank":     i + 1,
			}).Info("Found cached torrent")
			return &result, nil
		}

	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"checked":  len(filteredResults),
	}).Info("No cached torrents found")

	return nil, nil
}

// filterBlacklistedTorrents filters out blacklisted torrents from search results
func (s *TorrentService) filterBlacklistedTorrents(results []models.TorrentSearchResult) ([]models.TorrentSearchResult, error) {
	blacklist, err := s.getBlacklist()
	if err != nil {
		return nil, fmt.Errorf("getting blacklist: %w", err)
	}

	var filteredResults []models.TorrentSearchResult
	for _, result := range results {
		if !s.isBlacklisted(result.Title, blacklist) {
			filteredResults = append(filteredResults, result)
		}
	}

	return filteredResults, nil
}

func (s *TorrentService) sortTorrentResults(results []models.TorrentSearchResult) {
	utils.SortTorrentResultsByQuality(results)
}

// processBatchConcurrently processes a batch of media with controlled concurrency
func (s *TorrentService) processBatchConcurrently(ctx context.Context, medias []*models.Media, maxConcurrency int) error {
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	errChan := make(chan error, len(medias))

	for _, media := range medias {
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)
		go func(m *models.Media) {
			defer wg.Done()
			defer func() {
				select {
				case <-semaphore:
				default:
				}
			}()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			if ctx.Err() != nil {
				errChan <- ctx.Err()
				return
			}

			if err := s.populateTorrentsForMedia(m); err != nil {
				log.WithError(err).WithField("trakt_id", m.Trakt).Error("Failed to populate torrents")
				errChan <- err
			}
		}(media)
	}

	wg.Wait()
	close(errChan)

	var errorCount int
	for range errChan {
		errorCount++
	}

	if errorCount > 0 {
		log.WithField("error_count", errorCount).Warn("Some operations failed")
	}

	return ctx.Err()
}

// populateTorrentsForMedia populates torrent entries for a specific media item
func (s *TorrentService) populateTorrentsForMedia(media *models.Media) error {
	// Determine media type for search
	mediaType := "movie"
	if media.IsEpisode() {
		mediaType = "series"
	}

	// Search for torrents with year validation for movies
	var results []models.TorrentSearchResult
	var err error

	if media.IsMovie() && media.Year > 0 {
		// For movies, use year-aware search with Trakt slug
		results, err = s.searchService.SearchWithYearAndTraktSlug(
			media.IMDB,
			media.Title,
			mediaType,
			int(media.Season),
			int(media.Number),
			int(media.Year),
			media.TraktSlug,
		)
	} else {
		// For TV shows or movies without year, use regular search with Trakt slug
		results, err = s.searchService.SearchWithYearAndTraktSlug(
			media.IMDB,
			media.Title,
			mediaType,
			int(media.Season),
			int(media.Number),
			0,
			media.TraktSlug,
		)
	}

	if err != nil {
		return fmt.Errorf("searching torrents for media %d: %w", media.Trakt, err)
	}

	if len(results) == 0 {
		return nil
	}

	if err := s.insertTorrentResults(media, results); err != nil {
		return fmt.Errorf("inserting torrent results: %w", err)
	}

	return nil
}

// insertTorrentResults inserts torrent search results into the database
func (s *TorrentService) insertTorrentResults(media *models.Media, results []models.TorrentSearchResult) error {
	blacklist, err := s.getBlacklist()
	if err != nil {
		return fmt.Errorf("getting blacklist: %w", err)
	}

	var savedCount int
	var filteredCount int

	for _, result := range results {
		if s.isBlacklisted(result.Title, blacklist) {
			filteredCount++
			continue
		}

		if media.IsMovie() && media.Year > 0 && !result.MatchesYear(int(media.Year)) {
			filteredCount++
			continue
		}

		torrent := &models.Torrent{
			Trakt:        media.Trakt,
			Hash:         result.Hash,
			Title:        result.Title,
			Size:         result.Size,
			IsSeasonPack: result.IsSeasonPack(),
			Failed:       false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if torrent.IsSeasonPack {
			torrent.Season = result.ExtractSeason()
		}

		if err := s.repo.SaveTorrent(torrent); err != nil {
			log.WithError(err).WithField("title", torrent.Title).Error("Failed to save torrent")
			continue
		}

		savedCount++
	}

	if savedCount > 0 {
		log.WithFields(log.Fields{
			"trakt_id": media.Trakt,
			"count":    savedCount,
		}).Info("Saved torrents")
	}

	return nil
}

// getBlacklist retrieves the blacklist with thread-safe caching
func (s *TorrentService) getBlacklist() (map[string]struct{}, error) {
	s.blacklistMu.RLock()
	if len(s.blacklistCache) > 0 {
		// Return a copy to prevent external modifications
		cachedMap := make(map[string]struct{}, len(s.blacklistCache))
		for k := range s.blacklistCache {
			cachedMap[k] = struct{}{}
		}
		s.blacklistMu.RUnlock()
		return cachedMap, nil
	}
	s.blacklistMu.RUnlock()

	s.blacklistMu.Lock()
	defer s.blacklistMu.Unlock()

	// Double-check after acquiring write lock
	if len(s.blacklistCache) > 0 {
		cachedMap := make(map[string]struct{}, len(s.blacklistCache))
		for k := range s.blacklistCache {
			cachedMap[k] = struct{}{}
		}
		return cachedMap, nil
	}

	blacklistWords, err := s.readBlacklist()
	if err != nil {
		return nil, err
	}

	// Convert to map for O(1) lookups
	blacklistMap := make(map[string]struct{}, len(blacklistWords))
	for _, word := range blacklistWords {
		blacklistMap[strings.ToLower(word)] = struct{}{}
	}

	s.blacklistCache = blacklistMap

	// Return a copy
	result := make(map[string]struct{}, len(blacklistMap))
	for k := range blacklistMap {
		result[k] = struct{}{}
	}
	return result, nil
}

// readBlacklist reads the blacklist file
func (s *TorrentService) readBlacklist() ([]string, error) {
	file, err := os.Open(s.blacklistFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.WithField("file", s.blacklistFile).Debug("Blacklist file not found")
			return []string{}, nil
		}
		return nil, fmt.Errorf("opening blacklist file: %w", err)
	}
	defer file.Close()

	var blacklist []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			blacklist = append(blacklist, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning blacklist file: %w", err)
	}

	return blacklist, nil
}

// isBlacklisted checks if a title is blacklisted
func (s *TorrentService) isBlacklisted(title string, blacklist map[string]struct{}) bool {
	titleLower := strings.ToLower(title)

	for word := range blacklist {
		if strings.Contains(titleLower, word) {
			return true
		}
	}
	return false
}

// MarkTorrentFailed marks a torrent as failed
func (s *TorrentService) MarkTorrentFailed(traktID int64) error {
	torrent, err := s.repo.GetBestTorrent(traktID)
	if err != nil {
		return fmt.Errorf("getting torrent: %w", err)
	}

	torrent.MarkFailed()
	if err := s.repo.SaveTorrent(torrent); err != nil {
		return fmt.Errorf("marking torrent as failed: %w", err)
	}

	log.WithFields(log.Fields{
		"trakt_id": traktID,
		"title":    torrent.Title,
	}).Info("Marked torrent as failed")

	return nil
}

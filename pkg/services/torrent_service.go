package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

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

// CreateTorrentService creates a TorrentService
func CreateTorrentService(repo repository.Repository, blacklistFile string) *TorrentService {
	return &TorrentService{
		repo:           repo,
		searchService:  CreateTorrentSearchService(),
		blacklistFile:  blacklistFile,
		blacklistCache: make(map[string]struct{}),
	}
}

// NewTorrentService is deprecated, use CreateTorrentService
func NewTorrentService(repo repository.Repository, blacklistFile string) *TorrentService {
	return CreateTorrentService(repo, blacklistFile)
}

// CreateTorrentServiceWithTrakt creates a TorrentService with Trakt support
func CreateTorrentServiceWithTrakt(repo repository.Repository, blacklistFile string, traktService *TraktService) *TorrentService {
	return &TorrentService{
		repo:           repo,
		searchService:  CreateTorrentSearchServiceWithTrakt(traktService),
		blacklistFile:  blacklistFile,
		blacklistCache: make(map[string]struct{}),
	}
}

// NewTorrentServiceWithTrakt is deprecated, use CreateTorrentServiceWithTrakt
func NewTorrentServiceWithTrakt(repo repository.Repository, blacklistFile string, traktService *TraktService) *TorrentService {
	return CreateTorrentServiceWithTrakt(repo, blacklistFile, traktService)
}

// NewTorrentServiceWithTraktAndTMDB is deprecated, use CreateTorrentServiceWithTraktAndTMDB
func NewTorrentServiceWithTraktAndTMDB(repo repository.Repository, blacklistFile string, traktService *TraktService, tmdbService *TMDBService) *TorrentService {
	return CreateTorrentServiceWithTraktAndTMDB(repo, blacklistFile, traktService, tmdbService)
}

// CreateTorrentServiceWithTraktAndTMDB creates a TorrentService with Trakt and TMDB support
func CreateTorrentServiceWithTraktAndTMDB(repo repository.Repository, blacklistFile string, traktService *TraktService, tmdbService *TMDBService) *TorrentService {
	return &TorrentService{
		repo:           repo,
		searchService:  CreateTorrentSearchServiceWithTraktAndTMDB(traktService, tmdbService),
		blacklistFile:  blacklistFile,
		blacklistCache: make(map[string]struct{}),
	}
}

// NewTorrentServiceWithTraktAndTMDBAndOrionoid is deprecated, kept for backward compatibility
func NewTorrentServiceWithTraktAndTMDBAndOrionoid(repo repository.Repository, blacklistFile string, traktService *TraktService, tmdbService *TMDBService, _, _ string) *TorrentService {
	return CreateTorrentServiceWithTraktAndTMDB(repo, blacklistFile, traktService, tmdbService)
}

// GetBestTorrent is no longer supported since torrents are not stored in database
func (s *TorrentService) GetBestTorrent(traktID int64) (interface{}, error) {
	return nil, fmt.Errorf("torrent database functionality has been removed")
}

// GetSortedTorrents is no longer supported since torrents are not stored in database
func (s *TorrentService) GetSortedTorrents(traktID int64) ([]interface{}, error) {
	return nil, fmt.Errorf("torrent database functionality has been removed")
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
func (s *TorrentService) FindBestCachedTorrent(media *models.Media, allDebridService AllDebridInterface) (*models.TorrentSearchResult, error) {
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
		"tmdb_id":    media.TMDBID,
	}).Info("Starting torrent search (using stored database data)")

	// Check if we have stored original language data
	if media.OriginalLanguage != "" {
		log.WithFields(log.Fields{
			"original_language": media.OriginalLanguage,
			"french_title":      media.FrenchTitle,
		}).Info("Using stored TMDB data from database (no API calls)")
		
		// Use stored language for provider selection (preferred method)
		if media.IsMovie() && media.Year > 0 {
			// For movies, use year-aware search with stored language
			results, err = s.searchService.SearchWithLanguageAndFrenchTitle(
				media.Title,
				mediaType,
				int(media.Season),
				int(media.Number),
				int(media.Year),
				media.TMDBID,
				media.OriginalLanguage,
				media.FrenchTitle,
			)
		} else {
			// For TV shows or movies without year, use stored language search
			results, err = s.searchService.SearchWithLanguageAndFrenchTitle(
				media.Title,
				mediaType,
				int(media.Season),
				int(media.Number),
				0,
				media.TMDBID,
				media.OriginalLanguage,
				media.FrenchTitle,
			)
		}
	} else {
		// Fallback to basic search if TMDB not available
		if media.IsMovie() && media.Year > 0 {
			// For movies, use year-aware search
			results, err = s.searchService.SearchWithYear(
				media.Title,
				mediaType,
				int(media.Season),
				int(media.Number),
				int(media.Year),
			)
		} else {
			// For TV shows or movies without year, use basic search
			results, err = s.searchService.Search(
				media.Title,
				mediaType,
				int(media.Season),
				int(media.Number),
			)
		}
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

	// Group results by provider
	yggResults := []models.TorrentSearchResult{}
	apiBayResults := []models.TorrentSearchResult{}
	
	for _, result := range filteredResults {
		switch result.Source {
		case "YGG":
			yggResults = append(yggResults, result)
		case "APIBay":
			apiBayResults = append(apiBayResults, result)
		}
	}
	
	// Sort YGG results by size (biggest first)
	for i := 0; i < len(yggResults); i++ {
		for j := i + 1; j < len(yggResults); j++ {
			if yggResults[j].Size > yggResults[i].Size {
				yggResults[i], yggResults[j] = yggResults[j], yggResults[i]
			}
		}
	}
	
	log.WithFields(log.Fields{
		"trakt_id":       media.Trakt,
		"ygg_count":      len(yggResults),
		"apibay_count":   len(apiBayResults),
		"total_count":    len(filteredResults),
	}).Info("Checking AllDebrid cache")

	// Try YGG results first (biggest to smallest)
	for i, result := range yggResults {
		if result.Hash == "" {
			continue
		}

		log.WithFields(log.Fields{
			"provider": "YGG",
			"rank":     i + 1,
			"hash":     result.Hash,
			"title":    result.Title,
			"size_gb":  fmt.Sprintf("%.2f", float64(result.Size)/(1024*1024*1024)),
		}).Info("Checking YGG torrent")

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
	
	// Try APIBay results
	for i, result := range apiBayResults {
		if result.Hash == "" {
			continue
		}

		log.WithFields(log.Fields{
			"provider": "APIBay",
			"rank":     i + 1,
			"hash":     result.Hash,
			"title":    result.Title,
		}).Info("Checking APIBay torrent")

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

	// Check if we have stored original language data
	if media.OriginalLanguage != "" {
		// Use stored language for provider selection (preferred method)
		if media.IsMovie() && media.Year > 0 {
			// For movies, use year-aware search with stored language
			results, err = s.searchService.SearchWithLanguageAndFrenchTitle(
				media.Title,
				mediaType,
				int(media.Season),
				int(media.Number),
				int(media.Year),
				media.TMDBID,
				media.OriginalLanguage,
				media.FrenchTitle,
			)
		} else {
			// For TV shows or movies without year, use stored language search
			results, err = s.searchService.SearchWithLanguageAndFrenchTitle(
				media.Title,
				mediaType,
				int(media.Season),
				int(media.Number),
				0,
				media.TMDBID,
				media.OriginalLanguage,
				media.FrenchTitle,
			)
		}
	} else {
		// Fallback to basic search if TMDB not available
		if media.IsMovie() && media.Year > 0 {
			// For movies, use year-aware search
			results, err = s.searchService.SearchWithYear(
				media.Title,
				mediaType,
				int(media.Season),
				int(media.Number),
				int(media.Year),
			)
		} else {
			// For TV shows or movies without year, use basic search
			results, err = s.searchService.Search(
				media.Title,
				mediaType,
				int(media.Season),
				int(media.Number),
			)
		}
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

// insertTorrentResults is no longer needed since torrents are not stored in database
func (s *TorrentService) insertTorrentResults(media *models.Media, results []models.TorrentSearchResult) error {
	log.WithField("trakt_id", media.Trakt).Debug("insertTorrentResults called but no-op since torrents not stored in database")
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

// MarkTorrentFailed is no longer supported since torrents are not stored in database
func (s *TorrentService) MarkTorrentFailed(traktID int64) error {
	log.WithField("trakt_id", traktID).Info("MarkTorrentFailed called but no-op since torrents not stored in database")
	return nil
}

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
	mediaType := s.getMediaType(media)
	s.logSearchStart(media, mediaType)

	results, err := s.searchTorrents(media, mediaType)
	if err != nil {
		return nil, fmt.Errorf("searching torrents for media %d: %w", media.Trakt, err)
	}

	if len(results) == 0 {
		log.WithField("trakt_id", media.Trakt).Info("No torrents found")
		return nil, nil
	}

	filteredResults, err := s.applyBlacklistFilter(results, media)
	if err != nil || len(filteredResults) == 0 {
		return nil, err
	}

	return s.findCachedFromResults(filteredResults, media, allDebridService)
}

// getMediaType determines if media is movie or series
func (s *TorrentService) getMediaType(media *models.Media) string {
	if media.IsEpisode() {
		return "series"
	}
	return "movie"
}

// logSearchStart logs the start of torrent search
func (s *TorrentService) logSearchStart(media *models.Media, mediaType string) {
	log.WithFields(log.Fields{
		"trakt_id":   media.Trakt,
		"title":      media.Title,
		"media_type": mediaType,
		"season":     media.Season,
		"number":     media.Number,
		"year":       media.Year,
		"tmdb_id":    media.TMDBID,
	}).Info("Starting torrent search (using stored database data)")
}

// searchTorrents performs the torrent search
func (s *TorrentService) searchTorrents(media *models.Media, mediaType string) ([]models.TorrentSearchResult, error) {
	if media.OriginalLanguage != "" {
		return s.searchWithLanguage(media, mediaType)
	}
	return s.searchBasic(media, mediaType)
}

// searchWithLanguage searches using stored language data
func (s *TorrentService) searchWithLanguage(media *models.Media, mediaType string) ([]models.TorrentSearchResult, error) {
	log.WithFields(log.Fields{
		"original_language": media.OriginalLanguage,
		"french_title":      media.FrenchTitle,
	}).Info("Using stored TMDB data from database (no API calls)")

	yearParam := 0
	if media.IsMovie() && media.Year > 0 {
		yearParam = int(media.Year)
	}

	return s.searchService.SearchWithLanguageAndFrenchTitle(
		media.Title,
		mediaType,
		int(media.Season),
		int(media.Number),
		yearParam,
		media.TMDBID,
		media.OriginalLanguage,
		media.FrenchTitle,
	)
}

// searchBasic performs basic search without language data
func (s *TorrentService) searchBasic(media *models.Media, mediaType string) ([]models.TorrentSearchResult, error) {
	if media.IsMovie() && media.Year > 0 {
		return s.searchService.SearchWithYear(
			media.Title,
			mediaType,
			int(media.Season),
			int(media.Number),
			int(media.Year),
		)
	}

	return s.searchService.Search(
		media.Title,
		mediaType,
		int(media.Season),
		int(media.Number),
	)
}

// applyBlacklistFilter filters out blacklisted torrents
func (s *TorrentService) applyBlacklistFilter(results []models.TorrentSearchResult, media *models.Media) ([]models.TorrentSearchResult, error) {
	filteredResults, err := s.filterBlacklistedTorrents(results)
	if err != nil {
		return nil, fmt.Errorf("filtering blacklisted torrents: %w", err)
	}

	if len(filteredResults) == 0 {
		log.WithField("trakt_id", media.Trakt).Info("All torrents blacklisted")
		return nil, nil
	}

	return filteredResults, nil
}

// findCachedFromResults finds cached torrent from results
func (s *TorrentService) findCachedFromResults(results []models.TorrentSearchResult, media *models.Media, allDebridService AllDebridInterface) (*models.TorrentSearchResult, error) {
	yggResults, apiBayResults := s.groupResultsByProvider(results)
	s.sortYGGBySize(yggResults)
	s.logCacheCheck(media, len(yggResults), len(apiBayResults), len(results))

	// Try YGG results first
	if cached := s.findCachedInProvider(yggResults, "YGG", media, allDebridService); cached != nil {
		return cached, nil
	}

	// Try APIBay results
	if cached := s.findCachedInProvider(apiBayResults, "APIBay", media, allDebridService); cached != nil {
		return cached, nil
	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"checked":  len(results),
	}).Info("No cached torrents found")

	return nil, nil
}

// groupResultsByProvider groups results by provider
func (s *TorrentService) groupResultsByProvider(results []models.TorrentSearchResult) ([]models.TorrentSearchResult, []models.TorrentSearchResult) {
	var yggResults, apiBayResults []models.TorrentSearchResult

	for _, result := range results {
		switch result.Source {
		case "YGG":
			yggResults = append(yggResults, result)
		case "APIBay":
			apiBayResults = append(apiBayResults, result)
		}
	}

	return yggResults, apiBayResults
}

// sortYGGBySize sorts YGG results by size (biggest first)
func (s *TorrentService) sortYGGBySize(results []models.TorrentSearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Size > results[i].Size {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// logCacheCheck logs cache check start
func (s *TorrentService) logCacheCheck(media *models.Media, yggCount, apiBayCount, totalCount int) {
	log.WithFields(log.Fields{
		"trakt_id":     media.Trakt,
		"ygg_count":    yggCount,
		"apibay_count": apiBayCount,
		"total_count":  totalCount,
	}).Info("Checking AllDebrid cache")
}

// findCachedInProvider checks for cached torrents in a provider
func (s *TorrentService) findCachedInProvider(results []models.TorrentSearchResult, provider string, media *models.Media, allDebridService AllDebridInterface) *models.TorrentSearchResult {
	for i, result := range results {
		if result.Hash == "" {
			continue
		}

		s.logCheckingTorrent(provider, i+1, result)

		cached, _, err := allDebridService.IsTorrentCached(result.Hash)
		if err != nil {
			log.WithError(err).WithField("hash", result.Hash).Error("Failed to check AllDebrid cache")
			continue
		}

		if cached {
			s.logFoundCached(media, result, i+1)
			return &result
		}
	}
	return nil
}

// logCheckingTorrent logs torrent check
func (s *TorrentService) logCheckingTorrent(provider string, rank int, result models.TorrentSearchResult) {
	fields := log.Fields{
		"provider": provider,
		"rank":     rank,
		"hash":     result.Hash,
		"title":    result.Title,
	}

	if provider == "YGG" {
		fields["size_gb"] = fmt.Sprintf("%.2f", float64(result.Size)/(1024*1024*1024))
	}

	log.WithFields(fields).Info(fmt.Sprintf("Checking %s torrent", provider))
}

// logFoundCached logs when cached torrent is found
func (s *TorrentService) logFoundCached(media *models.Media, result models.TorrentSearchResult, rank int) {
	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    result.Title,
		"source":   result.Source,
		"rank":     rank,
	}).Info("Found cached torrent")
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
	mediaType := s.getMediaType(media)
	results, err := s.searchTorrents(media, mediaType)
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
	if cached := s.getCachedBlacklist(); cached != nil {
		return cached, nil
	}

	return s.loadAndCacheBlacklist()
}

// getCachedBlacklist returns cached blacklist if available
func (s *TorrentService) getCachedBlacklist() map[string]struct{} {
	s.blacklistMu.RLock()
	defer s.blacklistMu.RUnlock()

	if len(s.blacklistCache) > 0 {
		return s.copyBlacklistMap(s.blacklistCache)
	}
	return nil
}

// loadAndCacheBlacklist loads blacklist from file and caches it
func (s *TorrentService) loadAndCacheBlacklist() (map[string]struct{}, error) {
	s.blacklistMu.Lock()
	defer s.blacklistMu.Unlock()

	// Double-check after acquiring write lock
	if len(s.blacklistCache) > 0 {
		return s.copyBlacklistMap(s.blacklistCache), nil
	}

	blacklistWords, err := s.readBlacklist()
	if err != nil {
		return nil, err
	}

	blacklistMap := s.createBlacklistMap(blacklistWords)
	s.blacklistCache = blacklistMap

	return s.copyBlacklistMap(blacklistMap), nil
}

// copyBlacklistMap creates a copy of the blacklist map
func (s *TorrentService) copyBlacklistMap(source map[string]struct{}) map[string]struct{} {
	copy := make(map[string]struct{}, len(source))
	for k := range source {
		copy[k] = struct{}{}
	}
	return copy
}

// createBlacklistMap converts words to a map for O(1) lookups
func (s *TorrentService) createBlacklistMap(words []string) map[string]struct{} {
	blacklistMap := make(map[string]struct{}, len(words))
	for _, word := range words {
		blacklistMap[strings.ToLower(word)] = struct{}{}
	}
	return blacklistMap
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

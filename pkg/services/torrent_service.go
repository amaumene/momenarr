package services

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/amaumene/gostremiofr/pkg/alldebrid"
	"github.com/amaumene/gostremiofr/pkg/torrentsearch"
	tsmodels "github.com/amaumene/gostremiofr/pkg/torrentsearch/models"
	"github.com/amaumene/gostremiofr/pkg/torrentsearch/providers"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	"github.com/amaumene/momenarr/pkg/utils"
	log "github.com/sirupsen/logrus"
)

// TorrentService handles torrent search and management operations
type TorrentService struct {
	repo           repository.Repository
	searcher       *torrentsearch.TorrentSearch
	tmdbService    *TMDBService
	traktService   *TraktService
	blacklistFile  string
	blacklistCache map[string]struct{}
	blacklistMu    sync.RWMutex
}

// CreateTorrentService creates a TorrentService
func CreateTorrentService(repo repository.Repository, blacklistFile string) *TorrentService {
	searcher := torrentsearch.New(nil)
	searcher.RegisterProvider(providers.ProviderApiBay, providers.NewApiBayProvider())
	searcher.RegisterProvider(providers.ProviderYGG, providers.NewYGGProvider())
	
	return &TorrentService{
		repo:           repo,
		searcher:       searcher,
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
	searcher := torrentsearch.New(nil)
	searcher.RegisterProvider(providers.ProviderApiBay, providers.NewApiBayProvider())
	searcher.RegisterProvider(providers.ProviderYGG, providers.NewYGGProvider())
	
	return &TorrentService{
		repo:           repo,
		searcher:       searcher,
		traktService:   traktService,
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
	searcher := torrentsearch.New(nil)
	searcher.RegisterProvider(providers.ProviderApiBay, providers.NewApiBayProvider())
	searcher.RegisterProvider(providers.ProviderYGG, providers.NewYGGProvider())
	
	if tmdbService != nil && tmdbService.client != nil && tmdbService.apiKey != "" {
		searcher.SetTMDBAPIKey(tmdbService.apiKey)
	}
	
	return &TorrentService{
		repo:           repo,
		searcher:       searcher,
		tmdbService:    tmdbService,
		traktService:   traktService,
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
func (s *TorrentService) FindBestCachedTorrent(media *models.Media, allDebridClient *alldebrid.Client, apiKey string) (*models.TorrentSearchResult, error) {
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

	return s.findCachedFromResults(filteredResults, media, allDebridClient, apiKey)
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

	return s.performSearch(
		media.Title,
		mediaType,
		int(media.Season),
		int(media.Number),
		yearParam,
		media.OriginalLanguage,
		media.FrenchTitle,
	)
}

// searchBasic performs basic search without language data
func (s *TorrentService) searchBasic(media *models.Media, mediaType string) ([]models.TorrentSearchResult, error) {
	yearParam := 0
	if media.IsMovie() && media.Year > 0 {
		yearParam = int(media.Year)
	}

	return s.performSearch(
		media.Title,
		mediaType,
		int(media.Season),
		int(media.Number),
		yearParam,
		"",
		"",
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
func (s *TorrentService) findCachedFromResults(results []models.TorrentSearchResult, media *models.Media, allDebridClient *alldebrid.Client, apiKey string) (*models.TorrentSearchResult, error) {
	yggResults, apiBayResults := s.groupResultsByProvider(results)
	s.sortYGGBySize(yggResults)
	s.logCacheCheck(media, len(yggResults), len(apiBayResults), len(results))

	// Try YGG results first
	if cached := s.findCachedInProvider(yggResults, "YGG", media, allDebridClient, apiKey); cached != nil {
		return cached, nil
	}

	// Try APIBay results
	if cached := s.findCachedInProvider(apiBayResults, "APIBay", media, allDebridClient, apiKey); cached != nil {
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
func (s *TorrentService) findCachedInProvider(results []models.TorrentSearchResult, provider string, media *models.Media, allDebridClient *alldebrid.Client, apiKey string) *models.TorrentSearchResult {
	for i, result := range results {
		if result.Hash == "" {
			continue
		}

		s.logCheckingTorrent(provider, i+1, result)

		cached, _, err := s.isTorrentCached(allDebridClient, apiKey, result.Hash)
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

// performSearch executes the search and converts results
func (s *TorrentService) performSearch(title string, mediaType string, season, episode, year int, originalLanguage, frenchTitle string) ([]models.TorrentSearchResult, error) {
	options := tsmodels.SearchOptions{
		Query:           title,
		MediaType:       mediaType,
		Season:          season,
		Episode:         episode,
		Year:            year,
		Language:        originalLanguage,
		SpecificEpisode: episode > 0,
	}
	
	if s.searcher == nil {
		return nil, fmt.Errorf("searcher not initialized")
	}
	
	// Try smart search if TMDB is configured
	if s.tmdbService != nil && s.tmdbService.apiKey != "" {
		combined, _, err := s.searcher.SearchSmart(title, mediaType, season, episode, episode > 0)
		if err == nil {
			return s.convertCombinedResults(combined), nil
		}
		log.WithError(err).Error("Smart search failed, falling back to basic search")
	}
	
	// Fallback to basic search
	var allResults []models.TorrentSearchResult
	
	// Search APIBay
	if apiBayResults, err := s.searcher.Search(providers.ProviderApiBay, options); err == nil {
		allResults = append(allResults, s.convertSearchResults(apiBayResults)...)
	} else {
		log.WithError(err).Warn("APIBay search failed")
	}
	
	// Search YGG if appropriate
	if originalLanguage == "fr" || originalLanguage == "" {
		if yggResults, err := s.searcher.Search(providers.ProviderYGG, options); err == nil {
			allResults = append(allResults, s.convertSearchResults(yggResults)...)
		} else {
			log.WithError(err).Warn("YGG search failed")
		}
	}
	
	return allResults, nil
}

// convertCombinedResults converts CombinedSearchResults to TorrentSearchResult slice
func (s *TorrentService) convertCombinedResults(combined *tsmodels.CombinedSearchResults) []models.TorrentSearchResult {
	var results []models.TorrentSearchResult
	
	if combined == nil || combined.Results == nil {
		return results
	}
	
	for providerName, searchResults := range combined.Results {
		if searchResults == nil {
			continue
		}
		for _, torrent := range searchResults.MovieTorrents {
			results = append(results, s.convertTorrentInfo(torrent, providerName))
		}
		for _, torrent := range searchResults.CompleteSeriesTorrents {
			results = append(results, s.convertTorrentInfo(torrent, providerName))
		}
		for _, torrent := range searchResults.CompleteSeasonTorrents {
			results = append(results, s.convertTorrentInfo(torrent, providerName))
		}
		for _, torrent := range searchResults.EpisodeTorrents {
			results = append(results, s.convertTorrentInfo(torrent, providerName))
		}
	}
	
	return results
}

// convertSearchResults converts SearchResults to TorrentSearchResult slice
func (s *TorrentService) convertSearchResults(searchResults *tsmodels.SearchResults) []models.TorrentSearchResult {
	var results []models.TorrentSearchResult
	
	if searchResults == nil {
		return results
	}
	
	for _, torrent := range searchResults.MovieTorrents {
		results = append(results, s.convertTorrentInfo(torrent, torrent.Source))
	}
	
	for _, torrent := range searchResults.CompleteSeriesTorrents {
		results = append(results, s.convertTorrentInfo(torrent, torrent.Source))
	}
	
	for _, torrent := range searchResults.CompleteSeasonTorrents {
		results = append(results, s.convertTorrentInfo(torrent, torrent.Source))
	}
	
	for _, torrent := range searchResults.EpisodeTorrents {
		results = append(results, s.convertTorrentInfo(torrent, torrent.Source))
	}
	
	return results
}

// convertTorrentInfo converts TorrentInfo to TorrentSearchResult
func (s *TorrentService) convertTorrentInfo(torrent tsmodels.TorrentInfo, providerName string) models.TorrentSearchResult {
	source := torrent.Source
	if source == "" {
		source = s.mapProviderName(providerName)
	}
	
	magnetURL := ""
	if torrent.Hash != "" {
		magnetURL = fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", torrent.Hash, torrent.Title)
	}
	
	return models.TorrentSearchResult{
		Title:     torrent.Title,
		Hash:      torrent.Hash,
		Size:      torrent.Size,
		Seeders:   torrent.Seeders,
		Leechers:  torrent.Leechers,
		Source:    source,
		MagnetURL: magnetURL,
		ID:        torrent.ID,
	}
}

// mapProviderName maps provider constants to legacy source names
func (s *TorrentService) mapProviderName(providerName string) string {
	switch providerName {
	case providers.ProviderApiBay:
		return "APIBay"
	case providers.ProviderYGG:
		return "YGG"
	case providers.ProviderTorrentsCSV:
		return "TorrentsCSV"
	default:
		return providerName
	}
}

// isTorrentCached checks if a torrent is cached on AllDebrid
func (s *TorrentService) isTorrentCached(client *alldebrid.Client, apiKey string, hash string) (bool, int64, error) {
	magnetURL := fmt.Sprintf("magnet:?xt=urn:btih:%s", hash)
	
	uploadResult, err := client.UploadMagnet(apiKey, []string{magnetURL})
	if err != nil {
		return false, 0, fmt.Errorf("failed to upload magnet: %w", err)
	}
	
	if uploadResult.Error != nil {
		return false, 0, fmt.Errorf("upload error: %s", uploadResult.Error.Message)
	}
	
	if len(uploadResult.Data.Magnets) == 0 {
		return false, 0, nil
	}
	
	magnet := &uploadResult.Data.Magnets[0]
	if magnet.Error != nil {
		return false, 0, fmt.Errorf("magnet error: %s", magnet.Error.Message)
	}
	
	if magnet.Ready {
		return true, int64(magnet.ID), nil
	}
	
	// If not ready, delete it
	if err := client.DeleteMagnet(apiKey, strconv.FormatInt(magnet.ID, 10)); err != nil {
		log.WithError(err).Error("Failed to delete non-cached magnet")
	}
	
	return false, 0, nil
}

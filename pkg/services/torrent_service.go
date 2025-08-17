package services

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/amaumene/gostremiofr/pkg/alldebrid"
	"github.com/amaumene/gostremiofr/pkg/torrentsearch"
	tsmodels "github.com/amaumene/gostremiofr/pkg/torrentsearch/models"
	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/repository"
	log "github.com/sirupsen/logrus"
)


type TorrentService struct {
	repo             repository.Repository
	searcher         *torrentsearch.TorrentSearch
	tmdbService      *TMDBService
	traktService     *TraktService
	allDebridService *AllDebridService
	blacklistFile    string
	blacklistCache   map[string]struct{}
	blacklistMu      sync.RWMutex
}


func CreateTorrentServiceWithTrakt(repo repository.Repository, blacklistFile string, traktService *TraktService, allDebridClient *alldebrid.Client, apiKey string) *TorrentService {
	searcher := torrentsearch.New(nil)

	return &TorrentService{
		repo:             repo,
		searcher:         searcher,
		traktService:     traktService,
		allDebridService: NewAllDebridService(allDebridClient, apiKey),
		blacklistFile:    blacklistFile,
		blacklistCache:   make(map[string]struct{}),
	}
}


func CreateTorrentServiceWithTraktAndTMDB(repo repository.Repository, blacklistFile string, traktService *TraktService, tmdbService *TMDBService, allDebridClient *alldebrid.Client, apiKey string) *TorrentService {
	searcher := torrentsearch.New(nil)

	if tmdbService != nil && tmdbService.client != nil && tmdbService.apiKey != "" {
		searcher.SetTMDBAPIKey(tmdbService.apiKey)
	}

	return &TorrentService{
		repo:             repo,
		searcher:         searcher,
		tmdbService:      tmdbService,
		traktService:     traktService,
		allDebridService: NewAllDebridService(allDebridClient, apiKey),
		blacklistFile:    blacklistFile,
		blacklistCache:   make(map[string]struct{}),
	}
}


func (s *TorrentService) FindBestCachedTorrent(media *models.Media) (*models.TorrentSearchResult, error) {
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

	return s.findCachedFromResults(filteredResults, media)
}


func (s *TorrentService) getMediaType(media *models.Media) string {
	if media.IsEpisode() {
		return "series"
	}
	return "movie"
}


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


func (s *TorrentService) searchTorrents(media *models.Media, mediaType string) ([]models.TorrentSearchResult, error) {
	if media.OriginalLanguage != "" {
		return s.searchWithLanguage(media, mediaType)
	}
	return s.searchBasic(media, mediaType)
}


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


func (s *TorrentService) findCachedFromResults(results []models.TorrentSearchResult, media *models.Media) (*models.TorrentSearchResult, error) {
	s.sortResultsBySize(results)
	s.logCacheCheck(media, len(results))

	if cached := s.findCachedInResults(results, media); cached != nil {
		return cached, nil
	}

	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"checked":  len(results),
	}).Info("No cached torrents found")

	return nil, nil
}


func (s *TorrentService) sortResultsBySize(results []models.TorrentSearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Size > results[i].Size {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}


func (s *TorrentService) logCacheCheck(media *models.Media, totalCount int) {
	log.WithFields(log.Fields{
		"trakt_id":    media.Trakt,
		"total_count": totalCount,
	}).Info("Checking AllDebrid cache")
}


func (s *TorrentService) findCachedInResults(results []models.TorrentSearchResult, media *models.Media) *models.TorrentSearchResult {
	for i, result := range results {
		if result.Hash == "" {
			continue
		}

		s.logCheckingTorrent(i+1, result)

		cached, _, err := s.allDebridService.IsTorrentCached(result.Hash)
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


func (s *TorrentService) logCheckingTorrent(rank int, result models.TorrentSearchResult) {
	log.WithFields(log.Fields{
		"rank":    rank,
		"hash":    result.Hash,
		"title":   result.Title,
		"size_gb": fmt.Sprintf("%.2f", float64(result.Size)/(1024*1024*1024)),
	}).Info("Checking torrent")
}


func (s *TorrentService) logFoundCached(media *models.Media, result models.TorrentSearchResult, rank int) {
	log.WithFields(log.Fields{
		"trakt_id": media.Trakt,
		"title":    result.Title,
		"source":   result.Source,
		"rank":     rank,
	}).Info("Found cached torrent")
}


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


func (s *TorrentService) getBlacklist() (map[string]struct{}, error) {
	if cached := s.getCachedBlacklist(); cached != nil {
		return cached, nil
	}

	return s.loadAndCacheBlacklist()
}


func (s *TorrentService) getCachedBlacklist() map[string]struct{} {
	s.blacklistMu.RLock()
	defer s.blacklistMu.RUnlock()

	if len(s.blacklistCache) > 0 {
		return s.copyBlacklistMap(s.blacklistCache)
	}
	return nil
}


func (s *TorrentService) loadAndCacheBlacklist() (map[string]struct{}, error) {
	s.blacklistMu.Lock()
	defer s.blacklistMu.Unlock()


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


func (s *TorrentService) copyBlacklistMap(source map[string]struct{}) map[string]struct{} {
	copy := make(map[string]struct{}, len(source))
	for k := range source {
		copy[k] = struct{}{}
	}
	return copy
}


func (s *TorrentService) createBlacklistMap(words []string) map[string]struct{} {
	blacklistMap := make(map[string]struct{}, len(words))
	for _, word := range words {
		blacklistMap[strings.ToLower(word)] = struct{}{}
	}
	return blacklistMap
}


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


func (s *TorrentService) isBlacklisted(title string, blacklist map[string]struct{}) bool {
	titleLower := strings.ToLower(title)

	for word := range blacklist {
		if strings.Contains(titleLower, word) {
			return true
		}
	}
	return false
}


func (s *TorrentService) performSearch(title string, mediaType string, season, episode, year int, originalLanguage, frenchTitle string) ([]models.TorrentSearchResult, error) {
	if s.searcher == nil {
		return nil, fmt.Errorf("searcher not initialized")
	}


	if s.tmdbService != nil && s.tmdbService.apiKey != "" {
		combined, _, err := s.searcher.SearchSmart(title, mediaType, season, episode, episode > 0)
		if err == nil {
			return s.convertCombinedResults(combined), nil
		}
		log.WithError(err).Error("Smart search failed")
		return nil, fmt.Errorf("search failed: %w", err)
	}

	log.Warn("TMDB not configured, cannot perform search")
	return []models.TorrentSearchResult{}, nil
}

func (s *TorrentService) convertCombinedResults(combined *tsmodels.CombinedSearchResults) []models.TorrentSearchResult {
	var results []models.TorrentSearchResult

	if combined == nil || combined.Results == nil {
		return results
	}

	for providerName, searchResults := range combined.Results {
		results = s.appendProviderResults(results, searchResults, providerName)
	}

	return results
}

func (s *TorrentService) appendProviderResults(results []models.TorrentSearchResult, searchResults *tsmodels.SearchResults, providerName string) []models.TorrentSearchResult {
	if searchResults == nil {
		return results
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

	return results
}

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


func (s *TorrentService) mapProviderName(providerName string) string {
	if providerName == "" {
		return "Unknown"
	}
	return providerName
}

package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/amaumene/momenarr/pkg/models"
	"github.com/amaumene/momenarr/pkg/utils"
	log "github.com/sirupsen/logrus"
)

type TorrentSearchProvider interface {
	Search(query string, mediaType string, season, episode int) ([]models.TorrentSearchResult, error)
	GetName() string
}

type TorrentSearchService struct {
	providers    []TorrentSearchProvider
	httpClient   *http.Client
	traktService *TraktService
	tmdbService  *TMDBService
}

// CreateTorrentSearchService creates a basic torrent search service
func CreateTorrentSearchService() *TorrentSearchService {
	return createService(nil, nil)
}

// NewTorrentSearchService is deprecated, use CreateTorrentSearchService
func NewTorrentSearchService() *TorrentSearchService {
	return CreateTorrentSearchService()
}

// CreateTorrentSearchServiceWithTrakt creates a service with Trakt support
func CreateTorrentSearchServiceWithTrakt(traktService *TraktService) *TorrentSearchService {
	return createService(traktService, nil)
}

// NewTorrentSearchServiceWithTrakt is deprecated, use CreateTorrentSearchServiceWithTrakt
func NewTorrentSearchServiceWithTrakt(traktService *TraktService) *TorrentSearchService {
	return CreateTorrentSearchServiceWithTrakt(traktService)
}

// CreateTorrentSearchServiceWithTraktAndTMDB creates a service with Trakt and TMDB support
func CreateTorrentSearchServiceWithTraktAndTMDB(traktService *TraktService, tmdbService *TMDBService) *TorrentSearchService {
	return createService(traktService, tmdbService)
}

// NewTorrentSearchServiceWithTraktAndTMDB is deprecated, use CreateTorrentSearchServiceWithTraktAndTMDB
func NewTorrentSearchServiceWithTraktAndTMDB(traktService *TraktService, tmdbService *TMDBService) *TorrentSearchService {
	return CreateTorrentSearchServiceWithTraktAndTMDB(traktService, tmdbService)
}

// CreateTorrentSearchServiceWithTraktAndTMDB creates a service with Trakt and TMDB support
// Deprecated: Use CreateTorrentSearchServiceWithTraktAndTMDB instead
func NewTorrentSearchServiceWithTraktAndTMDBAndOrionoid(traktService *TraktService, tmdbService *TMDBService, _, _ string) *TorrentSearchService {
	return createService(traktService, tmdbService)
}

// createService is a helper to create the service with proper providers
func createService(traktService *TraktService, tmdbService *TMDBService) *TorrentSearchService {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	
	providers := []TorrentSearchProvider{
		CreateAPIBayProvider(httpClient),
	}
	
	if tmdbService != nil {
		providers = append(providers, CreateYGGProviderWithTMDB(httpClient, tmdbService))
	} else {
		providers = append(providers, CreateYGGProvider(httpClient))
	}
	
	return &TorrentSearchService{
		httpClient:   httpClient,
		traktService: traktService,
		tmdbService:  tmdbService,
		providers:    providers,
	}
}

// Search performs a basic search
func (s *TorrentSearchService) Search(title string, mediaType string, season, episode int) ([]models.TorrentSearchResult, error) {
	return s.searchWithFallback(title, mediaType, season, episode, 0)
}

// SearchWithYear performs a search including year
func (s *TorrentSearchService) SearchWithYear(title string, mediaType string, season, episode, year int) ([]models.TorrentSearchResult, error) {
	return s.searchWithFallback(title, mediaType, season, episode, year)
}

// SearchWithYearAndTraktID searches with Trakt ID for better matching
func (s *TorrentSearchService) SearchWithYearAndTraktID(title string, mediaType string, season, episode, year int, traktID int64) ([]models.TorrentSearchResult, error) {
	query := s.buildQuery(title, mediaType, season, episode, year, traktID)
	return s.searchAllProviders(query, mediaType, season, episode)
}

// SearchWithYearAndTMDB is deprecated - use SearchWithYear instead
func (s *TorrentSearchService) SearchWithYearAndTMDB(title string, mediaType string, season, episode, year int, tmdbID int64) ([]models.TorrentSearchResult, error) {
	return s.SearchWithYear(title, mediaType, season, episode, year)
}

// SearchWithLanguage performs language-aware search
func (s *TorrentSearchService) SearchWithLanguage(title string, mediaType string, season, episode, year int, tmdbID int64, originalLanguage string) ([]models.TorrentSearchResult, error) {
	return s.SearchWithLanguageAndFrenchTitle(title, mediaType, season, episode, year, tmdbID, originalLanguage, "")
}

// SearchWithLanguageAndFrenchTitle performs language-aware search with French title support
func (s *TorrentSearchService) SearchWithLanguageAndFrenchTitle(title string, mediaType string, season, episode, year int, tmdbID int64, originalLanguage string, frenchTitle string) ([]models.TorrentSearchResult, error) {
	provider, searchTitle := s.selectProviderByLanguage(originalLanguage, title, frenchTitle)
	if provider == nil {
		return nil, fmt.Errorf("no suitable provider found")
	}
	
	query := s.buildQuery(searchTitle, mediaType, season, episode, year, 0)
	
	log.WithFields(log.Fields{
		"provider": provider.GetName(),
		"language": originalLanguage,
		"query":    query,
	}).Debug("Performing language-aware search")
	
	results, err := s.performProviderSearch(provider, query, mediaType, season, episode, tmdbID, title)
	if err != nil {
		return nil, err
	}
	
	s.applySorting(provider.GetName(), results)
	return results, nil
}

// searchWithFallback performs search with fallback logic
func (s *TorrentSearchService) searchWithFallback(title string, mediaType string, season, episode, year int) ([]models.TorrentSearchResult, error) {
	provider := s.selectFallbackProvider()
	if provider == nil {
		return nil, fmt.Errorf("no providers available")
	}
	
	query := s.buildQuery(title, mediaType, season, episode, year, 0)
	
	results, err := provider.Search(query, mediaType, season, episode)
	if err != nil {
		log.WithError(err).WithField("provider", provider.GetName()).Error("Search failed")
		return nil, err
	}
	
	s.applySorting(provider.GetName(), results)
	return results, nil
}

// searchAllProviders searches across all available providers
func (s *TorrentSearchService) searchAllProviders(query string, mediaType string, season, episode int) ([]models.TorrentSearchResult, error) {
	var allResults []models.TorrentSearchResult
	
	for _, provider := range s.providers {
		results, err := provider.Search(query, mediaType, season, episode)
		if err != nil {
			log.WithError(err).WithField("provider", provider.GetName()).Warn("Provider search failed")
			continue
		}
		allResults = append(allResults, results...)
	}
	
	s.sortResults(allResults)
	return allResults, nil
}

// selectProviderByLanguage selects the best provider based on content language
func (s *TorrentSearchService) selectProviderByLanguage(language, title, frenchTitle string) (TorrentSearchProvider, string) {
	isFrench := language == "fr"
	searchTitle := title
	
	if isFrench && frenchTitle != "" {
		searchTitle = frenchTitle
	}
	
	targetProvider := "APIBay"
	if isFrench {
		targetProvider = "YGG"
	}
	
	// Find preferred provider
	for _, provider := range s.providers {
		if provider.GetName() == targetProvider {
			return provider, searchTitle
		}
	}
	
	// Fallback to any available provider
	if len(s.providers) > 0 {
		log.WithField("target", targetProvider).Warn("Preferred provider not found, using fallback")
		return s.providers[0], searchTitle
	}
	
	return nil, searchTitle
}

// selectFallbackProvider selects a provider for fallback searches
func (s *TorrentSearchService) selectFallbackProvider() TorrentSearchProvider {
	// Prefer APIBay for general content
	for _, provider := range s.providers {
		if provider.GetName() == "APIBay" {
			return provider
		}
	}
	
	// Fallback to first available
	if len(s.providers) > 0 {
		return s.providers[0]
	}
	
	return nil
}

// performProviderSearch executes search with provider-specific logic
func (s *TorrentSearchService) performProviderSearch(provider TorrentSearchProvider, query string, mediaType string, season, episode int, tmdbID int64, englishTitle string) ([]models.TorrentSearchResult, error) {
	// Special handling for YGG provider with French titles
	if provider.GetName() == "YGG" && tmdbID > 0 {
		if yggProvider, ok := provider.(*YGGProvider); ok {
			return yggProvider.SearchWithStoredFrenchTitle(query, mediaType, season, episode, tmdbID, englishTitle)
		}
	}
	
	return provider.Search(query, mediaType, season, episode)
}

// buildQuery constructs the search query
func (s *TorrentSearchService) buildQuery(title string, mediaType string, season, episode, year int, traktID int64) string {
	encodedTitle := strings.ReplaceAll(title, " ", "+")
	var query string
	
	switch mediaType {
	case "movie":
		if year > 0 {
			query = fmt.Sprintf("%s+%d", encodedTitle, year)
		} else {
			query = encodedTitle
		}
	case "series", "show":
		if season > 0 {
			query = fmt.Sprintf("%s+s%02d", encodedTitle, season)
		} else {
			query = encodedTitle
		}
	default:
		query = encodedTitle
	}
	
	// Add Trakt ID suffix if available
	if traktID > 0 {
		query = fmt.Sprintf("%s [%d]", query, traktID)
	}
	
	return query
}

// applySorting applies provider-specific sorting
func (s *TorrentSearchService) applySorting(providerName string, results []models.TorrentSearchResult) {
	switch providerName {
	case "YGG":
		s.sortResultsBySize(results)
	default:
		s.sortResults(results)
	}
}

// sortResults sorts by quality
func (s *TorrentSearchService) sortResults(results []models.TorrentSearchResult) {
	utils.SortTorrentResultsByQuality(results)
}

// sortResultsBySize sorts by size (largest first)
func (s *TorrentSearchService) sortResultsBySize(results []models.TorrentSearchResult) {
	utils.SortTorrentResultsBySize(results)
}
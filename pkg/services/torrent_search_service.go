package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
}

func NewTorrentSearchService() *TorrentSearchService {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	service := &TorrentSearchService{
		httpClient: httpClient,
		providers: []TorrentSearchProvider{
			NewAPIBayProvider(httpClient),
			NewYGGProvider(httpClient, nil), // No Trakt service
		},
	}

	return service
}

// NewTorrentSearchServiceWithTrakt creates a new torrent search service with Trakt support
func NewTorrentSearchServiceWithTrakt(traktService *TraktService) *TorrentSearchService {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	service := &TorrentSearchService{
		httpClient:   httpClient,
		traktService: traktService,
		providers: []TorrentSearchProvider{
			NewAPIBayProvider(httpClient),
			NewYGGProvider(httpClient, traktService),
		},
	}

	return service
}

func (s *TorrentSearchService) Search(imdbID string, title string, mediaType string, season, episode int) ([]models.TorrentSearchResult, error) {
	return s.SearchWithYearAndTraktID(imdbID, title, mediaType, season, episode, 0, 0)
}
func (s *TorrentSearchService) SearchWithYear(imdbID string, title string, mediaType string, season, episode, year int) ([]models.TorrentSearchResult, error) {
	return s.SearchWithYearAndTraktID(imdbID, title, mediaType, season, episode, year, 0)
}

func (s *TorrentSearchService) SearchWithYearAndTraktID(imdbID string, title string, mediaType string, season, episode, year int, traktID int64) ([]models.TorrentSearchResult, error) {
	var allResults []models.TorrentSearchResult

	// Build search query
	query := s.buildSearchQueryWithYearAndTraktID(title, mediaType, season, episode, year, traktID)

	for _, provider := range s.providers {
		results, err := provider.Search(query, mediaType, season, episode)
		if err != nil {
			log.WithError(err).WithField("provider", provider.GetName()).Error("Failed to search torrents")
			continue
		}
		allResults = append(allResults, results...)
	}

	// Sort by seeders and size
	s.sortResults(allResults)

	return allResults, nil
}

func (s *TorrentSearchService) SearchWithYearAndTraktSlug(imdbID string, title string, mediaType string, season, episode, year int, traktSlug string) ([]models.TorrentSearchResult, error) {
	var allResults []models.TorrentSearchResult

	// Build search query
	query := s.buildSearchQueryWithYearAndTraktSlug(title, mediaType, season, episode, year, traktSlug)

	log.WithFields(log.Fields{
		"query":      query,
		"media_type": mediaType,
		"season":     season,
		"episode":    episode,
		"year":       year,
		"trakt_slug": traktSlug,
		"providers":  len(s.providers),
	}).Info("Searching torrent providers")

	for _, provider := range s.providers {
		log.WithField("provider", provider.GetName()).Info("Searching provider")

		// Use the regular search method for all providers
		results, err := provider.Search(query, mediaType, season, episode)

		if err != nil {
			log.WithError(err).WithField("provider", provider.GetName()).Error("Failed to search torrents")
			continue
		}
		log.WithFields(log.Fields{
			"provider": provider.GetName(),
			"results":  len(results),
		}).Info("Provider search completed")
		allResults = append(allResults, results...)
	}

	// Sort by seeders and size
	s.sortResults(allResults)

	log.WithField("total_results", len(allResults)).Info("Torrent search completed")

	return allResults, nil
}

// buildSearchQuery builds the search query based on media type
func (s *TorrentSearchService) buildSearchQuery(title string, mediaType string, season, episode int) string {
	return s.buildSearchQueryWithParams(title, mediaType, season, episode, 0, "", 0)
}

// buildSearchQueryWithYear builds the search query based on media type including year for movies
func (s *TorrentSearchService) buildSearchQueryWithYear(title string, mediaType string, season, episode, year int) string {
	return s.buildSearchQueryWithParams(title, mediaType, season, episode, year, "", 0)
}

// buildSearchQueryWithYearAndTraktID builds the search query based on media type including year and traktID
func (s *TorrentSearchService) buildSearchQueryWithYearAndTraktID(title string, mediaType string, season, episode, year int, traktID int64) string {
	return s.buildSearchQueryWithParams(title, mediaType, season, episode, year, "", traktID)
}

// buildSearchQueryWithYearAndTraktSlug builds the search query based on media type including year and traktSlug
func (s *TorrentSearchService) buildSearchQueryWithYearAndTraktSlug(title string, mediaType string, season, episode, year int, traktSlug string) string {
	return s.buildSearchQueryWithParams(title, mediaType, season, episode, year, traktSlug, 0)
}

// buildSearchQueryWithParams builds the search query with all parameters
func (s *TorrentSearchService) buildSearchQueryWithParams(title string, mediaType string, season, episode, year int, traktSlug string, traktID int64) string {
	// Base query building
	encodedTitle := strings.ReplaceAll(title, " ", "+")
	var baseQuery string

	if mediaType == "movie" {
		// For movies, include year in search
		if year > 0 {
			baseQuery = fmt.Sprintf("%s+%d", encodedTitle, year)
		} else {
			baseQuery = encodedTitle
		}
	} else if season > 0 {
		// For TV shows, search by season to get both episodes and season packs
		baseQuery = fmt.Sprintf("%s+s%02d", encodedTitle, season)
	} else {
		baseQuery = encodedTitle
	}

	// Append suffix for providers that need it (like YGG for French translation)
	if traktID > 0 {
		return fmt.Sprintf("%s [%d]", baseQuery, traktID)
	}
	if traktSlug != "" {
		return fmt.Sprintf("%s [%s]", baseQuery, traktSlug)
	}

	return baseQuery
}

func (s *TorrentSearchService) sortResults(results []models.TorrentSearchResult) {
	utils.SortTorrentResultsByQuality(results)
}

// APIBayProvider implements torrent search using The Pirate Bay API
type APIBayProvider struct {
	httpClient *http.Client
	baseURL    string
}

// NewAPIBayProvider creates a new APIBay provider
func NewAPIBayProvider(httpClient *http.Client) *APIBayProvider {
	return &APIBayProvider{
		httpClient: httpClient,
		baseURL:    "https://apibay.org",
	}
}

// GetName returns the provider name
func (p *APIBayProvider) GetName() string {
	return "APIBay"
}

// Search searches for torrents on APIBay
func (p *APIBayProvider) Search(query string, mediaType string, season, episode int) ([]models.TorrentSearchResult, error) {
	// Build API URL
	apiURL := fmt.Sprintf("%s/q.php?q=%s&cat=video", p.baseURL, url.QueryEscape(query))

	// Make request
	resp, err := p.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search APIBay: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("APIBay returned status %d", resp.StatusCode)
	}

	// Parse response
	var apiResults []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		InfoHash string `json:"info_hash"`
		Seeders  string `json:"seeders"`
		Leechers string `json:"leechers"`
		Size     string `json:"size"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResults); err != nil {
		return nil, fmt.Errorf("failed to decode APIBay response: %w", err)
	}

	// Convert to our format
	var results []models.TorrentSearchResult
	for _, apiResult := range apiResults {
		// Skip if no info hash
		if apiResult.InfoHash == "" || apiResult.InfoHash == "0" {
			continue
		}

		// Filter results based on media type (less restrictive)
		if mediaType == "series" && season > 0 {
			// For TV series, accept both episode matches and season packs
			// Don't filter too aggressively - let the torrent service decide later
			if episode > 0 {
				// For specific episodes, accept if it matches the episode OR is a season pack
				if !p.matchesEpisode(apiResult.Name, season, episode) && !p.isSeasonPack(apiResult.Name, season) {
					// Also accept if it just mentions the season (less strict)
					if !p.mentionsSeason(apiResult.Name, season) {
						continue
					}
				}
			} else {
				// For season searches, accept anything that mentions the season
				if !p.mentionsSeason(apiResult.Name, season) {
					continue
				}
			}
		}

		seeders, _ := strconv.Atoi(apiResult.Seeders)
		leechers, _ := strconv.Atoi(apiResult.Leechers)
		size, _ := strconv.ParseInt(apiResult.Size, 10, 64)

		result := models.TorrentSearchResult{
			Title:     apiResult.Name,
			Hash:      strings.ToLower(apiResult.InfoHash),
			Size:      size,
			Seeders:   seeders,
			Leechers:  leechers,
			Source:    "APIBay",
			MagnetURL: fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", apiResult.InfoHash, url.QueryEscape(apiResult.Name)),
		}

		results = append(results, result)
	}

	return results, nil
}

// matchesEpisode checks if a torrent title matches a specific episode
func (p *APIBayProvider) matchesEpisode(title string, season, episode int) bool {
	lowerTitle := strings.ToLower(title)

	// Check common episode patterns
	patterns := []string{
		fmt.Sprintf("s%02de%02d", season, episode),
		fmt.Sprintf("s%de%d", season, episode),
		fmt.Sprintf("%dx%02d", season, episode),
		fmt.Sprintf("season %d episode %d", season, episode),
	}

	for _, pattern := range patterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}

	return false
}

// isSeasonPack checks if a torrent is a complete season pack
func (p *APIBayProvider) isSeasonPack(title string, season int) bool {
	lowerTitle := strings.ToLower(title)

	// Check if it mentions the season but not a specific episode
	seasonPatterns := []string{
		fmt.Sprintf("season %d", season),
		fmt.Sprintf("s%02d", season),
		fmt.Sprintf("s%d", season),
	}

	hasSeasonMention := false
	for _, pattern := range seasonPatterns {
		if strings.Contains(lowerTitle, pattern) {
			hasSeasonMention = true
			break
		}
	}

	if !hasSeasonMention {
		return false
	}

	// Check for season pack indicators
	packIndicators := []string{
		"complete",
		"full season",
		"season pack",
		"all episodes",
	}

	for _, indicator := range packIndicators {
		if strings.Contains(lowerTitle, indicator) {
			return true
		}
	}

	// If it mentions season but not a specific episode, it's likely a pack
	episodePatterns := []string{
		fmt.Sprintf("s%02de", season),
		fmt.Sprintf("s%de", season),
		fmt.Sprintf("%dx", season),
		fmt.Sprintf("episode"),
	}

	for _, pattern := range episodePatterns {
		if strings.Contains(lowerTitle, pattern) {
			return false
		}
	}

	return true
}

// mentionsSeason checks if a torrent title mentions a specific season (less strict)
func (p *APIBayProvider) mentionsSeason(title string, season int) bool {
	lowerTitle := strings.ToLower(title)

	// Check various season patterns
	seasonPatterns := []string{
		fmt.Sprintf("season %d", season),
		fmt.Sprintf("s%02d", season),
		fmt.Sprintf("s%d", season),
		fmt.Sprintf("season.%d", season),
		fmt.Sprintf("s.%d", season),
	}

	for _, pattern := range seasonPatterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}

	return false
}

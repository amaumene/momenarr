package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/amaumene/momenarr/pkg/models"
	log "github.com/sirupsen/logrus"
)

const (
	// YGG API endpoints
	yggAPIBase         = "https://yggapi.eu"
	yggSearchEndpoint  = "/torrents"
	yggTorrentEndpoint = "/torrent"

	// YGG category IDs
	movieCategories  = "&category_id=2178&category_id=2181&category_id=2183"
	seriesCategories = "&category_id=2179&category_id=2181&category_id=2182&category_id=2184"

	// API parameters
	defaultPage              = 1
	defaultPerPage           = 100
	maxConcurrentHashFetches = 5
)

// YGGProvider implements torrent search using YggTorrent API
type YGGProvider struct {
	httpClient   *http.Client
	hashCache    sync.Map // Cache for torrent hashes
	traktService *TraktService
}

// YggTorrent represents a torrent from YGG API
type YggTorrent struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Size     int64  `json:"size"`
	Seeders  int    `json:"seeders"`
	Leechers int    `json:"leechers"`
	Hash     string `json:"hash,omitempty"`
}

// NewYGGProvider creates a new YGG provider
func NewYGGProvider(httpClient *http.Client, traktService *TraktService) *YGGProvider {
	return &YGGProvider{
		httpClient:   httpClient,
		traktService: traktService,
	}
}

// GetName returns the provider name
func (p *YGGProvider) GetName() string {
	return "YGG"
}

// Search searches for torrents on YGG (legacy method for backward compatibility)
func (p *YGGProvider) Search(query string, mediaType string, season, episode int) ([]models.TorrentSearchResult, error) {
	// For legacy calls, try to extract Trakt slug from query if it's provided
	// Format: "Title Year [traktSlug]" or "Title+Year+[traktSlug]"
	frenchQuery := query
	var traktSlug string

	if p.traktService != nil {
		// Try to extract Trakt slug from query if it's provided
		if idx := strings.LastIndex(query, "["); idx != -1 {
			if endIdx := strings.Index(query[idx:], "]"); endIdx != -1 {
				traktSlug = query[idx+1 : idx+endIdx]
				if traktSlug != "" {
					// Remove the Trakt slug from query
					queryWithoutSlug := strings.TrimSpace(query[:idx])

					originalTitle := queryWithoutSlug
					frenchQuery = p.traktService.GetFrenchTitle(originalTitle, mediaType, traktSlug)

					if frenchQuery != originalTitle {
						log.WithFields(log.Fields{
							"original": originalTitle,
							"french":   frenchQuery,
						}).Info("Using French title for search")
					}
				}
			}
		}
	}

	// Build search query using the translated title
	searchQuery := p.buildSearchQuery(frenchQuery, mediaType, season, episode)

	return p.performSearch(searchQuery, mediaType, season, episode)
}

// performSearch performs the actual YGG API search
func (p *YGGProvider) performSearch(searchQuery string, mediaType string, season, episode int) ([]models.TorrentSearchResult, error) {
	// Build API URL
	encodedQuery := url.QueryEscape(searchQuery)
	categoryParams := p.getCategoryParams(mediaType)
	apiURL := fmt.Sprintf("%s%s?q=%s&page=%d&per_page=%d%s",
		yggAPIBase, yggSearchEndpoint, encodedQuery, defaultPage, defaultPerPage, categoryParams)

	log.WithFields(log.Fields{
		"search_query": searchQuery,
		"media_type":   mediaType,
		"season":       season,
		"episode":      episode,
		"api_url":      apiURL,
	}).Info("Searching YGG API")

	resp, err := p.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search YGG: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.WithField("status_code", resp.StatusCode).Warn("YGG API returned non-OK status")
		return []models.TorrentSearchResult{}, nil
	}

	// Read the response body to check if it's valid JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read YGG response: %w", err)
	}

	// Check if response looks like JSON
	if len(body) > 0 && body[0] != '[' && body[0] != '{' {
		log.Warn("YGG API returned invalid JSON response")
		return []models.TorrentSearchResult{}, nil
	}

	var yggTorrents []YggTorrent
	if err := json.Unmarshal(body, &yggTorrents); err != nil {
		log.WithError(err).Warn("Failed to decode YGG response")
		return []models.TorrentSearchResult{}, nil
	}

	log.WithField("raw_results", len(yggTorrents)).Info("YGG API returned results")

	// Convert to our format
	var results []models.TorrentSearchResult
	for _, yggTorrent := range yggTorrents {
		// Filter based on media type
		if mediaType == "series" && season > 0 {
			if episode > 0 {
				// For specific episodes, accept if it matches the episode OR is a season pack
				if !p.matchesEpisode(yggTorrent.Title, season, episode) && !p.isSeasonPack(yggTorrent.Title, season) {
					if !p.mentionsSeason(yggTorrent.Title, season) {
						continue
					}
				}
			} else {
				// For season searches, accept anything that mentions the season
				if !p.mentionsSeason(yggTorrent.Title, season) {
					continue
				}
			}
		}

		result := models.TorrentSearchResult{
			Title:     yggTorrent.Title,
			Hash:      "", // Will be fetched later if needed
			Size:      yggTorrent.Size,
			Seeders:   yggTorrent.Seeders,
			Leechers:  yggTorrent.Leechers,
			Source:    "YGG",
			MagnetURL: "",                          // Will be generated after hash is fetched
			ID:        strconv.Itoa(yggTorrent.ID), // Store ID for later hash fetching
		}

		results = append(results, result)
	}

	log.WithField("filtered_results", len(results)).Info("YGG search filtering completed")

	// Fetch hashes for results with good seeders (top 10)
	if len(results) > 0 {
		p.fetchHashesForTopResults(results)
	}

	return results, nil
}

// buildSearchQuery builds the search query following gostremiofr pattern
func (p *YGGProvider) buildSearchQuery(query string, mediaType string, season, episode int) string {
	// The query from torrent search service already includes season/year info
	// Just clean it up and return it as-is to avoid duplication

	// Replace spaces with + for URL encoding
	cleanQuery := strings.ReplaceAll(query, " ", "+")

	// Remove any URL encoding that might already be present
	cleanQuery = strings.ReplaceAll(cleanQuery, "%2B", "+")

	return cleanQuery
}

// getCategoryParams returns the appropriate category parameters for the content type
func (p *YGGProvider) getCategoryParams(mediaType string) string {
	switch mediaType {
	case "movie":
		return movieCategories
	case "series":
		return seriesCategories
	default:
		return ""
	}
}

// fetchHashesForTopResults fetches hashes for torrents with the most seeders
func (p *YGGProvider) fetchHashesForTopResults(results []models.TorrentSearchResult) {
	// Only fetch hashes for top 10 results to avoid too many API calls
	limit := 10
	if len(results) < limit {
		limit = len(results)
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentHashFetches)

	for i := 0; i < limit; i++ {
		if results[i].ID == "" {
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := &results[idx]
			hash, err := p.getTorrentHash(result.ID)
			if err != nil {
				log.WithError(err).WithField("torrent_id", result.ID).Error("Failed to get torrent hash")
				return
			}

			result.Hash = hash
			result.MagnetURL = fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", hash, url.QueryEscape(result.Title))
		}(i)
	}

	wg.Wait()
}

// getTorrentHash fetches the hash for a specific torrent
func (p *YGGProvider) getTorrentHash(torrentID string) (string, error) {
	// Check cache first
	if cached, ok := p.hashCache.Load(torrentID); ok {
		if hash, ok := cached.(string); ok {
			return hash, nil
		}
	}

	apiURL := fmt.Sprintf("%s%s/%s", yggAPIBase, yggTorrentEndpoint, torrentID)

	resp, err := p.httpClient.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to get torrent hash: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("YGG API returned status %d for torrent %s", resp.StatusCode, torrentID)
	}

	var result struct {
		Hash string `json:"hash"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode hash response: %w", err)
	}

	// Cache the result
	p.hashCache.Store(torrentID, result.Hash)

	return result.Hash, nil
}

// matchesEpisode checks if a torrent title matches a specific episode
func (p *YGGProvider) matchesEpisode(title string, season, episode int) bool {
	lowerTitle := strings.ToLower(title)

	// Check common episode patterns
	patterns := []string{
		fmt.Sprintf("s%02de%02d", season, episode),
		fmt.Sprintf("s%de%d", season, episode),
		fmt.Sprintf("%dx%02d", season, episode),
		fmt.Sprintf("season %d episode %d", season, episode),
		fmt.Sprintf("saison %d episode %d", season, episode), // French
		fmt.Sprintf("saison %d épisode %d", season, episode), // French with accent
	}

	for _, pattern := range patterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}

	return false
}

// isSeasonPack checks if a torrent is a complete season pack
func (p *YGGProvider) isSeasonPack(title string, season int) bool {
	lowerTitle := strings.ToLower(title)

	// Check if it mentions the season but not a specific episode
	seasonPatterns := []string{
		fmt.Sprintf("season %d", season),
		fmt.Sprintf("saison %d", season), // French
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
		"complet",  // French
		"complète", // French
		"full season",
		"saison complete", // French
		"saison complète", // French
		"season pack",
		"all episodes",
		"tous les épisodes", // French
		"integrale",         // French
		"intégrale",         // French
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
		"episode",
		"épisode", // French
	}

	for _, pattern := range episodePatterns {
		if strings.Contains(lowerTitle, pattern) {
			return false
		}
	}

	return true
}

// mentionsSeason checks if a torrent title mentions a specific season
func (p *YGGProvider) mentionsSeason(title string, season int) bool {
	lowerTitle := strings.ToLower(title)

	// Check various season patterns
	seasonPatterns := []string{
		fmt.Sprintf("season %d", season),
		fmt.Sprintf("saison %d", season), // French
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

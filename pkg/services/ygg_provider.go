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
	yggAPIBase         = "https://yggapi.eu"
	yggSearchEndpoint  = "/torrents"
	yggTorrentEndpoint = "/torrent"

	movieCategories  = "&category_id=2178&category_id=2181&category_id=2183"
	seriesCategories = "&category_id=2179&category_id=2181&category_id=2182&category_id=2184"

	defaultPage              = 1
	defaultPerPage           = 100
	maxConcurrentHashFetches = 5
	maxHashFetchResults      = 10
)

// YggTorrent represents a torrent from YGG API
type YggTorrent struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Size     int64  `json:"size"`
	Seeders  int    `json:"seeders"`
	Leechers int    `json:"leechers"`
	Hash     string `json:"hash,omitempty"`
}

// YGGProvider searches torrents on YggTorrent (French tracker)
type YGGProvider struct {
	httpClient  *http.Client
	hashCache   sync.Map
	tmdbService *TMDBService
}

// CreateYGGProvider creates a YGG provider
func CreateYGGProvider(httpClient *http.Client) *YGGProvider {
	return &YGGProvider{
		httpClient: httpClient,
	}
}

// NewYGGProvider is deprecated, use CreateYGGProvider
func NewYGGProvider(httpClient *http.Client) *YGGProvider {
	return &YGGProvider{
		httpClient: httpClient,
	}
}

// CreateYGGProviderWithTMDB creates a YGG provider with TMDB support
func CreateYGGProviderWithTMDB(httpClient *http.Client, tmdbService *TMDBService) *YGGProvider {
	return &YGGProvider{
		httpClient:  httpClient,
		tmdbService: tmdbService,
	}
}

// NewYGGProviderWithTMDB is deprecated, use CreateYGGProviderWithTMDB
func NewYGGProviderWithTMDB(httpClient *http.Client, tmdbService *TMDBService) *YGGProvider {
	return &YGGProvider{
		httpClient:  httpClient,
		tmdbService: tmdbService,
	}
}

// GetName returns the provider name
func (p *YGGProvider) GetName() string {
	return "YGG"
}

// Search searches for torrents on YGG
func (p *YGGProvider) Search(query string, mediaType string, season, episode int) ([]models.TorrentSearchResult, error) {
	torrents, err := p.fetchTorrents(query, mediaType)
	if err != nil {
		return nil, err
	}

	results := p.filterAndConvert(torrents, mediaType, season, episode)
	p.enrichTopResults(results)

	return results, nil
}

// SearchWithStoredFrenchTitle searches using pre-fetched French title
func (p *YGGProvider) SearchWithStoredFrenchTitle(query string, mediaType string, season, episode int, tmdbID int64, originalTitle string) ([]models.TorrentSearchResult, error) {
	// For backward compatibility - just use the provided query
	return p.Search(query, mediaType, season, episode)
}

// fetchTorrents retrieves torrents from YGG API
func (p *YGGProvider) fetchTorrents(query string, mediaType string) ([]YggTorrent, error) {
	apiURL := p.buildAPIURL(query, mediaType)

	log.WithFields(log.Fields{
		"query": query,
		"type":  mediaType,
	}).Debug("Fetching from YGG API")

	resp, err := p.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("YGG API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.WithField("status", resp.StatusCode).Warn("YGG API returned non-OK status")
		return []YggTorrent{}, nil
	}

	return p.parseResponse(resp.Body)
}

// buildAPIURL constructs the API endpoint URL
func (p *YGGProvider) buildAPIURL(query string, mediaType string) string {
	encodedQuery := url.QueryEscape(query)
	categories := p.getCategoryParams(mediaType)

	return fmt.Sprintf("%s%s?q=%s&page=%d&per_page=%d%s",
		yggAPIBase, yggSearchEndpoint, encodedQuery,
		defaultPage, defaultPerPage, categories)
}

// parseResponse parses the API response
func (p *YGGProvider) parseResponse(body io.Reader) ([]YggTorrent, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Validate JSON format
	if len(data) > 0 && data[0] != '[' && data[0] != '{' {
		log.Warn("YGG API returned invalid JSON")
		return []YggTorrent{}, nil
	}

	var torrents []YggTorrent
	if err := json.Unmarshal(data, &torrents); err != nil {
		log.WithError(err).Warn("Failed to parse YGG response")
		return []YggTorrent{}, nil
	}

	return torrents, nil
}

// filterAndConvert filters torrents and converts to search results
func (p *YGGProvider) filterAndConvert(torrents []YggTorrent, mediaType string, season, episode int) []models.TorrentSearchResult {
	var results []models.TorrentSearchResult

	for _, torrent := range torrents {
		if !p.matchesMediaFilter(torrent.Title, mediaType, season, episode) {
			continue
		}

		results = append(results, p.convertToSearchResult(torrent))
	}

	return results
}

// matchesMediaFilter checks if torrent matches the search criteria
func (p *YGGProvider) matchesMediaFilter(title, mediaType string, season, episode int) bool {
	if mediaType != "series" || season == 0 {
		return true
	}

	if episode > 0 {
		return p.matchesEpisode(title, season, episode) ||
			p.isSeasonPack(title, season) ||
			p.containsSeason(title, season)
	}

	return p.containsSeason(title, season)
}

// convertToSearchResult converts YGG torrent to search result
func (p *YGGProvider) convertToSearchResult(torrent YggTorrent) models.TorrentSearchResult {
	return models.TorrentSearchResult{
		Title:     torrent.Title,
		Hash:      "",
		Size:      torrent.Size,
		Seeders:   torrent.Seeders,
		Leechers:  torrent.Leechers,
		Source:    p.GetName(),
		MagnetURL: "",
		ID:        strconv.Itoa(torrent.ID),
	}
}

// enrichTopResults fetches hashes for top results
func (p *YGGProvider) enrichTopResults(results []models.TorrentSearchResult) {
	limit := min(len(results), maxHashFetchResults)
	if limit == 0 {
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentHashFetches)

	for i := 0; i < limit; i++ {
		if results[i].ID == "" {
			continue
		}

		wg.Add(1)
		go p.enrichResultWithHash(&results[i], &wg, semaphore)
	}

	wg.Wait()
}

// enrichResultWithHash fetches and sets hash for a single result
func (p *YGGProvider) enrichResultWithHash(result *models.TorrentSearchResult, wg *sync.WaitGroup, sem chan struct{}) {
	defer wg.Done()

	sem <- struct{}{}
	defer func() { <-sem }()

	hash, err := p.fetchTorrentHash(result.ID)
	if err != nil {
		log.WithError(err).WithField("id", result.ID).Debug("Failed to fetch hash")
		return
	}

	result.Hash = hash
	result.MagnetURL = p.buildMagnetURL(hash, result.Title)
}

// fetchTorrentHash retrieves hash for a specific torrent
func (p *YGGProvider) fetchTorrentHash(torrentID string) (string, error) {
	// Check cache
	if cached, ok := p.hashCache.Load(torrentID); ok {
		return cached.(string), nil
	}

	apiURL := fmt.Sprintf("%s%s/%s", yggAPIBase, yggTorrentEndpoint, torrentID)

	resp, err := p.httpClient.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("hash fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result struct {
		Hash string `json:"hash"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode hash: %w", err)
	}

	// Cache result
	p.hashCache.Store(torrentID, result.Hash)

	return result.Hash, nil
}

// buildMagnetURL constructs a magnet link
func (p *YGGProvider) buildMagnetURL(hash, title string) string {
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", hash, url.QueryEscape(title))
}

// getCategoryParams returns category filters for the media type
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

// matchesEpisode checks if title matches a specific episode
func (p *YGGProvider) matchesEpisode(title string, season, episode int) bool {
	lowerTitle := strings.ToLower(title)
	patterns := p.getEpisodePatterns(season, episode)

	for _, pattern := range patterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}
	return false
}

// getEpisodePatterns returns episode naming patterns (including French)
func (p *YGGProvider) getEpisodePatterns(season, episode int) []string {
	return []string{
		fmt.Sprintf("s%02de%02d", season, episode),
		fmt.Sprintf("s%de%d", season, episode),
		fmt.Sprintf("%dx%02d", season, episode),
		fmt.Sprintf("season %d episode %d", season, episode),
		fmt.Sprintf("saison %d episode %d", season, episode),
		fmt.Sprintf("saison %d épisode %d", season, episode),
	}
}

// isSeasonPack checks if title is a complete season pack
func (p *YGGProvider) isSeasonPack(title string, season int) bool {
	if !p.containsSeason(title, season) {
		return false
	}

	lowerTitle := strings.ToLower(title)

	// Check for pack indicators
	packIndicators := []string{
		"complete", "complet", "complète",
		"full season", "saison complete", "saison complète",
		"season pack", "all episodes", "tous les épisodes",
		"integrale", "intégrale",
	}

	for _, indicator := range packIndicators {
		if strings.Contains(lowerTitle, indicator) {
			return true
		}
	}

	// If mentions season but not episode, likely a pack
	episodeIndicators := []string{
		fmt.Sprintf("s%02de", season),
		fmt.Sprintf("s%de", season),
		fmt.Sprintf("%dx", season),
		"episode", "épisode",
	}

	for _, indicator := range episodeIndicators {
		if strings.Contains(lowerTitle, indicator) {
			return false
		}
	}

	return true
}

// containsSeason checks if title mentions a specific season
func (p *YGGProvider) containsSeason(title string, season int) bool {
	lowerTitle := strings.ToLower(title)
	patterns := p.getSeasonPatterns(season)

	for _, pattern := range patterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}
	return false
}

// getSeasonPatterns returns season naming patterns (including French)
func (p *YGGProvider) getSeasonPatterns(season int) []string {
	return []string{
		fmt.Sprintf("season %d", season),
		fmt.Sprintf("saison %d", season),
		fmt.Sprintf("s%02d", season),
		fmt.Sprintf("s%d", season),
		fmt.Sprintf("season.%d", season),
		fmt.Sprintf("s.%d", season),
	}
}

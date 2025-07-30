package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/amaumene/momenarr/pkg/models"
)

type apiBayResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	InfoHash string `json:"info_hash"`
	Seeders  string `json:"seeders"`
	Leechers string `json:"leechers"`
	Size     string `json:"size"`
}

// APIBayProvider searches torrents using The Pirate Bay API
type APIBayProvider struct {
	httpClient *http.Client
	baseURL    string
}

// CreateAPIBayProvider creates an APIBay provider
func CreateAPIBayProvider(httpClient *http.Client) *APIBayProvider {
	return &APIBayProvider{
		httpClient: httpClient,
		baseURL:    "https://apibay.org",
	}
}

// NewAPIBayProvider is deprecated, use CreateAPIBayProvider
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
	apiResults, err := p.fetchResults(query)
	if err != nil {
		return nil, err
	}
	
	return p.filterAndConvert(apiResults, mediaType, season, episode), nil
}

// fetchResults retrieves raw results from APIBay
func (p *APIBayProvider) fetchResults(query string) ([]apiBayResponse, error) {
	apiURL := fmt.Sprintf("%s/q.php?q=%s&cat=video", p.baseURL, url.QueryEscape(query))
	
	resp, err := p.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("APIBay request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("APIBay returned status %d", resp.StatusCode)
	}
	
	var results []apiBayResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode APIBay response: %w", err)
	}
	
	return results, nil
}

// filterAndConvert filters API results and converts to our model
func (p *APIBayProvider) filterAndConvert(apiResults []apiBayResponse, mediaType string, season, episode int) []models.TorrentSearchResult {
	var results []models.TorrentSearchResult
	
	for _, apiResult := range apiResults {
		if !p.isValidResult(apiResult) {
			continue
		}
		
		if !p.matchesMediaFilter(apiResult.Name, mediaType, season, episode) {
			continue
		}
		
		results = append(results, p.convertToSearchResult(apiResult))
	}
	
	return results
}

// isValidResult checks if the result has valid data
func (p *APIBayProvider) isValidResult(result apiBayResponse) bool {
	return result.InfoHash != "" && result.InfoHash != "0"
}

// matchesMediaFilter checks if result matches the media type criteria
func (p *APIBayProvider) matchesMediaFilter(title, mediaType string, season, episode int) bool {
	if mediaType != "series" || season == 0 {
		return true // No filtering for movies or general searches
	}
	
	// For series with specific episode
	if episode > 0 {
		return p.matchesEpisode(title, season, episode) || 
		       p.isSeasonPack(title, season) || 
		       p.containsSeason(title, season)
	}
	
	// For season searches
	return p.containsSeason(title, season)
}

// convertToSearchResult converts API response to our model
func (p *APIBayProvider) convertToSearchResult(apiResult apiBayResponse) models.TorrentSearchResult {
	seeders, _ := strconv.Atoi(apiResult.Seeders)
	leechers, _ := strconv.Atoi(apiResult.Leechers)
	size, _ := strconv.ParseInt(apiResult.Size, 10, 64)
	
	return models.TorrentSearchResult{
		Title:     apiResult.Name,
		Hash:      strings.ToLower(apiResult.InfoHash),
		Size:      size,
		Seeders:   seeders,
		Leechers:  leechers,
		Source:    p.GetName(),
		MagnetURL: p.buildMagnetURL(apiResult.InfoHash, apiResult.Name),
	}
}

// buildMagnetURL constructs a magnet link
func (p *APIBayProvider) buildMagnetURL(infoHash, name string) string {
	return fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", infoHash, url.QueryEscape(name))
}

// matchesEpisode checks if title matches a specific episode
func (p *APIBayProvider) matchesEpisode(title string, season, episode int) bool {
	lowerTitle := strings.ToLower(title)
	patterns := p.getEpisodePatterns(season, episode)
	
	for _, pattern := range patterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}
	return false
}

// getEpisodePatterns returns common episode naming patterns
func (p *APIBayProvider) getEpisodePatterns(season, episode int) []string {
	return []string{
		fmt.Sprintf("s%02de%02d", season, episode),
		fmt.Sprintf("s%de%d", season, episode),
		fmt.Sprintf("%dx%02d", season, episode),
		fmt.Sprintf("season %d episode %d", season, episode),
	}
}

// isSeasonPack checks if title is a complete season pack
func (p *APIBayProvider) isSeasonPack(title string, season int) bool {
	lowerTitle := strings.ToLower(title)
	
	if !p.containsSeason(title, season) {
		return false
	}
	
	// Check for pack indicators
	packIndicators := []string{"complete", "full season", "season pack", "all episodes"}
	for _, indicator := range packIndicators {
		if strings.Contains(lowerTitle, indicator) {
			return true
		}
	}
	
	// If mentions season but not specific episode, likely a pack
	episodePatterns := []string{
		fmt.Sprintf("s%02de", season),
		fmt.Sprintf("s%de", season),
		fmt.Sprintf("%dx", season),
		"episode",
	}
	
	for _, pattern := range episodePatterns {
		if strings.Contains(lowerTitle, pattern) {
			return false
		}
	}
	
	return true
}

// containsSeason checks if title mentions a specific season
func (p *APIBayProvider) containsSeason(title string, season int) bool {
	lowerTitle := strings.ToLower(title)
	patterns := p.getSeasonPatterns(season)
	
	for _, pattern := range patterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}
	return false
}

// getSeasonPatterns returns common season naming patterns
func (p *APIBayProvider) getSeasonPatterns(season int) []string {
	return []string{
		fmt.Sprintf("season %d", season),
		fmt.Sprintf("s%02d", season),
		fmt.Sprintf("s%d", season),
		fmt.Sprintf("season.%d", season),
		fmt.Sprintf("s.%d", season),
	}
}
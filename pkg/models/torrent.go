package models

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)


// TorrentSearchResult represents a torrent search result
type TorrentSearchResult struct {
	Title     string
	Hash      string
	Size      int64
	Seeders   int
	Leechers  int
	Source    string
	MagnetURL string
	ID        string // Provider-specific ID (e.g., for YGG to fetch hash later)
}

// IsSeasonPack checks if the torrent title indicates a complete season
func (r *TorrentSearchResult) IsSeasonPack() bool {
	lowerTitle := strings.ToLower(r.Title)

	// Check for season pack indicators
	seasonPackPatterns := []string{
		"complete season",
		"full season",
		"season pack",
		"complete series",
		"integrale", // French
		"completa",  // Spanish/Italian
	}

	for _, pattern := range seasonPackPatterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}

	// Check for season without episode (e.g., "S01" but not "S01E01")
	if regexp.MustCompile(`(?i)s\d{2}(?:[^e]|$)`).MatchString(r.Title) {
		// Make sure it's not a single episode
		if !regexp.MustCompile(`(?i)s\d{2}e\d{2}`).MatchString(r.Title) {
			return true
		}
	}

	return false
}

// ExtractSeason extracts the season number from the title
func (r *TorrentSearchResult) ExtractSeason() int {
	// Try to extract season number from patterns like S01, Season 1, etc.
	patterns := []string{
		`(?i)s(\d{1,2})(?:[^e]|$)`,
		`(?i)season\s*(\d{1,2})`,
		`(?i)saison\s*(\d{1,2})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(r.Title)
		if len(matches) > 1 {
			season, err := strconv.Atoi(matches[1])
			if err == nil {
				return season
			}
		}
	}

	return 0
}

// ExtractYear extracts year from the torrent title
func (r *TorrentSearchResult) ExtractYear() int {
	// Look for 4-digit year patterns (1900-2099)
	yearPattern := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	matches := yearPattern.FindAllString(r.Title, -1)

	if len(matches) > 0 {
		// Take the first year found
		if year, err := strconv.Atoi(matches[0]); err == nil {
			return year
		}
	}

	return 0
}

// MatchesYear checks if the torrent title contains a year that matches the expected year
func (r *TorrentSearchResult) MatchesYear(expectedYear int) bool {
	if expectedYear == 0 {
		return true // If no year provided, don't filter
	}

	// Look for 4-digit year patterns in the title
	yearPattern := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	matches := yearPattern.FindAllString(r.Title, -1)

	for _, match := range matches {
		if year, err := strconv.Atoi(match); err == nil {
			// Allow exact year match or year+1 (some torrents are released early)
			if year == expectedYear || year == expectedYear+1 {
				return true
			}
		}
	}

	// For movies, be more strict about year matching
	// Only allow missing year for very recent movies (within 2 years)
	if len(matches) == 0 {
		currentYear := time.Now().Year()
		return expectedYear >= currentYear-2
	}

	return false
}

// IsRemux checks if the torrent is a remux (uncompressed/lossless rip)
func (r *TorrentSearchResult) IsRemux() bool {
	lowerTitle := strings.ToLower(r.Title)

	// Check for remux indicators
	return strings.Contains(lowerTitle, "remux")
}

// ExtractResolution extracts the resolution from the title and returns a numeric value for comparison
func (r *TorrentSearchResult) ExtractResolution() int {
	lowerTitle := strings.ToLower(r.Title)

	// Resolution priority mapping (higher number = better quality)
	resolutionMap := map[string]int{
		"8k":    8000,
		"4320p": 4320,
		"4k":    4000,
		"2160p": 2160,
		"1440p": 1440,
		"1080p": 1080,
		"720p":  720,
		"480p":  480,
		"360p":  360,
		"240p":  240,
	}

	// Check for resolution patterns
	for resolution, value := range resolutionMap {
		if strings.Contains(lowerTitle, resolution) {
			return value
		}
	}

	// Check for UHD indicators (4K equivalent)
	uhdPatterns := []string{"uhd", "ultra.hd", "ultra hd"}
	for _, pattern := range uhdPatterns {
		if strings.Contains(lowerTitle, pattern) {
			return 2160 // 4K UHD
		}
	}

	// Check for HD indicators (assume 1080p if not specified)
	hdPatterns := []string{"hd", "high.definition", "full.hd", "fhd"}
	for _, pattern := range hdPatterns {
		if strings.Contains(lowerTitle, pattern) {
			return 1080 // Assume 1080p
		}
	}

	// Default to SD quality if no resolution found
	return 480
}

package models

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TorrentSearchResult represents a torrent search result.
type TorrentSearchResult struct {
	Title     string
	Hash      string
	Size      int64
	Seeders   int
	Leechers  int
	Source    string
	MagnetURL string
	ID        string
}

var (
	seasonOnlyPattern    = regexp.MustCompile(`(?i)s\d{2}(?:[^e]|$)`)
	seasonEpisodePattern = regexp.MustCompile(`(?i)s\d{2}e\d{2}`)
	yearPattern          = regexp.MustCompile(`\b(19|20)\d{2}\b`)

	seasonPackPatterns = []string{
		"complete season",
		"full season",
		"season pack",
		"complete series",
		"integrale",
		"completa",
	}
)

// IsSeasonPack checks if the torrent title indicates a complete season.
func (r *TorrentSearchResult) IsSeasonPack() bool {
	lowerTitle := strings.ToLower(r.Title)

	for _, pattern := range seasonPackPatterns {
		if strings.Contains(lowerTitle, pattern) {
			return true
		}
	}

	if seasonOnlyPattern.MatchString(r.Title) {
		return !seasonEpisodePattern.MatchString(r.Title)
	}

	return false
}

var seasonPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)s(\d{1,2})(?:[^e]|$)`),
	regexp.MustCompile(`(?i)season\s*(\d{1,2})`),
	regexp.MustCompile(`(?i)saison\s*(\d{1,2})`),
}

// ExtractSeason extracts the season number from the title.
func (r *TorrentSearchResult) ExtractSeason() int {
	for _, pattern := range seasonPatterns {
		matches := pattern.FindStringSubmatch(r.Title)
		if len(matches) > 1 {
			season, err := strconv.Atoi(matches[1])
			if err == nil {
				return season
			}
		}
	}
	return 0
}

// ExtractYear extracts year from the torrent title.
func (r *TorrentSearchResult) ExtractYear() int {
	matches := yearPattern.FindAllString(r.Title, -1)
	if len(matches) == 0 {
		return 0
	}

	year, err := strconv.Atoi(matches[0])
	if err != nil {
		return 0
	}

	return year
}

// MatchesYear checks if the torrent title contains a year that matches the expected year.
func (r *TorrentSearchResult) MatchesYear(expectedYear int) bool {
	if expectedYear == 0 {
		return true
	}

	matches := yearPattern.FindAllString(r.Title, -1)

	for _, match := range matches {
		year, err := strconv.Atoi(match)
		if err != nil {
			continue
		}

		if year == expectedYear || year == expectedYear+1 {
			return true
		}
	}

	if len(matches) == 0 {
		currentYear := time.Now().Year()
		return expectedYear >= currentYear-2
	}

	return false
}

// IsRemux checks if the torrent is a remux (uncompressed/lossless rip).
func (r *TorrentSearchResult) IsRemux() bool {
	lowerTitle := strings.ToLower(r.Title)
	return strings.Contains(lowerTitle, "remux")
}

var resolutionMap = map[string]int{
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

const (
	resolution4K      = 2160
	resolution1080p   = 1080
	resolutionDefault = 480
)

// ExtractResolution extracts the resolution from the title.
func (r *TorrentSearchResult) ExtractResolution() int {
	lowerTitle := strings.ToLower(r.Title)

	for resolution, value := range resolutionMap {
		if strings.Contains(lowerTitle, resolution) {
			return value
		}
	}

	if containsAny(lowerTitle, "uhd", "ultra.hd", "ultra hd") {
		return resolution4K
	}

	if containsAny(lowerTitle, "hd", "high.definition", "full.hd", "fhd") {
		return resolution1080p
	}

	return resolutionDefault
}

func containsAny(s string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

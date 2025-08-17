package models

import (
	"regexp"
	"strings"
)


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

	seasonPackPatterns = []string{
		"complete season",
		"full season",
		"season pack",
		"complete series",
		"integrale",
		"completa",
	}
)


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

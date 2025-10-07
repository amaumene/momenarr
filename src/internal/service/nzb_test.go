package service

import (
	"testing"

	"github.com/amaumene/momenarr/internal/domain"
)

func TestIsBlacklisted(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		blacklist []string
		want      bool
	}{
		{
			name:      "not blacklisted",
			title:     "Movie.2023.1080p.BluRay",
			blacklist: []string{"CAM", "HDCAM"},
			want:      false,
		},
		{
			name:      "blacklisted - exact match",
			title:     "Movie.2023.CAM",
			blacklist: []string{"CAM", "HDCAM"},
			want:      true,
		},
		{
			name:      "blacklisted - case insensitive",
			title:     "Movie.2023.cam.xvid",
			blacklist: []string{"CAM", "HDCAM"},
			want:      true,
		},
		{
			name:      "blacklisted - partial match",
			title:     "Movie.2023.HDCAM.x264",
			blacklist: []string{"CAM", "HDCAM"},
			want:      true,
		},
		{
			name:      "empty blacklist",
			title:     "Movie.2023.CAM",
			blacklist: []string{},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlacklisted(tt.title, tt.blacklist)
			if got != tt.want {
				t.Errorf("isBlacklisted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateNZBKey(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{
			name:  "with prefix",
			title: "https://v2.nzbs.in/releases/abc123",
			want:  "abc123",
		},
		{
			name:  "without prefix",
			title: "xyz789",
			want:  "xyz789",
		},
		{
			name:  "empty string",
			title: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateNZBKey(tt.title)
			if got != tt.want {
				t.Errorf("generateNZBKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsEpisode(t *testing.T) {
	tests := []struct {
		name   string
		number int64
		season int64
		want   bool
	}{
		{
			name:   "valid episode",
			number: 1,
			season: 1,
			want:   true,
		},
		{
			name:   "zero number",
			number: 0,
			season: 1,
			want:   false,
		},
		{
			name:   "zero season",
			number: 1,
			season: 0,
			want:   false,
		},
		{
			name:   "both zero",
			number: 0,
			season: 0,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media := &domain.Media{
				Number: tt.number,
				Season: tt.season,
			}
			got := isEpisode(media)
			if got != tt.want {
				t.Errorf("isEpisode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSeasonPackTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{
			name:  "season pack with S notation",
			title: "Show.Name.S01.1080p.WEB-DL",
			want:  true,
		},
		{
			name:  "season pack with Season notation and space",
			title: "Show.Name.Season 1.1080p.BluRay",
			want:  true,
		},
		{
			name:  "season pack with Season notation and dot",
			title: "Show.Name.Season.1.1080p.BluRay",
			want:  true,
		},
		{
			name:  "season pack with single digit S notation",
			title: "Show.Name.S1.1080p.WEB-DL",
			want:  true,
		},
		{
			name:  "single episode S01E05",
			title: "Show.Name.S01E05.1080p.WEB-DL",
			want:  false,
		},
		{
			name:  "single episode with full notation",
			title: "Show.Name.S01E05.Episode.Title.1080p",
			want:  false,
		},
		{
			name:  "episode with dot before E",
			title: "Show.Name.S01.E01.1080p",
			want:  false,
		},
		{
			name:  "multi-episode pack",
			title: "Show.S01E01E02E03.1080p",
			want:  false,
		},
		{
			name:  "season pack S02 with COMPLETE",
			title: "Show.Name.S02.COMPLETE.1080p",
			want:  true,
		},
		{
			name:  "season pack with Season.01",
			title: "Show.Season.01.Complete",
			want:  true,
		},
		{
			name:  "movie without season notation",
			title: "Movie.Name.2023.1080p.BluRay",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSeasonPackTitle(tt.title)
			if got != tt.want {
				t.Errorf("isSeasonPackTitle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterSeasonPacks(t *testing.T) {
	tests := []struct {
		name    string
		results []domain.SearchResult
		want    int
	}{
		{
			name: "mixed results",
			results: []domain.SearchResult{
				{Title: "Show.S01.1080p.WEB-DL"},
				{Title: "Show.S01E05.1080p.WEB-DL"},
				{Title: "Show.Season 1.COMPLETE"},
				{Title: "Show.S01E06.720p"},
			},
			want: 2,
		},
		{
			name: "only season packs",
			results: []domain.SearchResult{
				{Title: "Show.S01.1080p.WEB-DL"},
				{Title: "Show.Season 1.COMPLETE"},
			},
			want: 2,
		},
		{
			name: "only episodes",
			results: []domain.SearchResult{
				{Title: "Show.S01E05.1080p.WEB-DL"},
				{Title: "Show.S01E06.720p"},
			},
			want: 0,
		},
		{
			name:    "empty results",
			results: []domain.SearchResult{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSeasonPacks(tt.results)
			if len(got) != tt.want {
				t.Errorf("filterSeasonPacks() returned %d results, want %d", len(got), tt.want)
			}
		})
	}
}

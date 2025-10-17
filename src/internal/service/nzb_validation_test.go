package service

import (
	"testing"

	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
)

func TestCalculateSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected float64
	}{
		{
			name:     "identical strings",
			a:        "breaking bad",
			b:        "breaking bad",
			expected: 1.0,
		},
		{
			name:     "similar strings",
			a:        "breaking bad",
			b:        "breaking good",
			expected: 0.7692307692307692,
		},
		{
			name:     "very different strings",
			a:        "breaking bad",
			b:        "walking dead",
			expected: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := calculateSimilarity(tt.a, tt.b)
			if similarity != tt.expected {
				t.Errorf("got %v, want %v", similarity, tt.expected)
			}
		})
	}
}

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name          string
		parsedTitle   string
		mediaTitle    string
		minSimilarity float64
		expectValid   bool
		expectScore   int
	}{
		{
			name:          "exact match",
			parsedTitle:   "breaking bad",
			mediaTitle:    "Breaking Bad",
			minSimilarity: 0.7,
			expectValid:   true,
			expectScore:   50,
		},
		{
			name:          "high similarity",
			parsedTitle:   "breaking bad",
			mediaTitle:    "Breaking Good",
			minSimilarity: 0.7,
			expectValid:   true,
			expectScore:   38,
		},
		{
			name:          "below threshold",
			parsedTitle:   "breaking bad",
			mediaTitle:    "Walking Dead",
			minSimilarity: 0.7,
			expectValid:   false,
			expectScore:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, score := validateTitle(tt.parsedTitle, tt.mediaTitle, tt.minSimilarity)
			if valid != tt.expectValid {
				t.Errorf("valid: got %v, want %v", valid, tt.expectValid)
			}
			if score != tt.expectScore {
				t.Errorf("score: got %v, want %v", score, tt.expectScore)
			}
		})
	}
}

func TestValidateYear(t *testing.T) {
	tests := []struct {
		name        string
		parsedYear  int64
		mediaYear   int64
		tolerance   int64
		isEpisode   bool
		expectValid bool
		expectScore int
	}{
		{
			name:        "movie: exact match",
			parsedYear:  2008,
			mediaYear:   2008,
			tolerance:   1,
			isEpisode:   false,
			expectValid: true,
			expectScore: 30,
		},
		{
			name:        "movie: one year off",
			parsedYear:  2009,
			mediaYear:   2008,
			tolerance:   1,
			isEpisode:   false,
			expectValid: true,
			expectScore: 20,
		},
		{
			name:        "movie: two years off",
			parsedYear:  2010,
			mediaYear:   2008,
			tolerance:   1,
			isEpisode:   false,
			expectValid: true,
			expectScore: 10,
		},
		{
			name:        "movie: outside tolerance",
			parsedYear:  2015,
			mediaYear:   2008,
			tolerance:   1,
			isEpisode:   false,
			expectValid: false,
			expectScore: 0,
		},
		{
			name:        "movie: no year in media",
			parsedYear:  2008,
			mediaYear:   0,
			tolerance:   1,
			isEpisode:   false,
			expectValid: true,
			expectScore: 30,
		},
		{
			name:        "movie: no year in parsed",
			parsedYear:  0,
			mediaYear:   2008,
			tolerance:   1,
			isEpisode:   false,
			expectValid: false,
			expectScore: 0,
		},
		{
			name:        "episode: exact match",
			parsedYear:  2008,
			mediaYear:   2008,
			tolerance:   1,
			isEpisode:   true,
			expectValid: true,
			expectScore: 30,
		},
		{
			name:        "episode: one year off should fail",
			parsedYear:  2009,
			mediaYear:   2008,
			tolerance:   1,
			isEpisode:   true,
			expectValid: false,
			expectScore: 0,
		},
		{
			name:        "episode: no year in parsed should pass",
			parsedYear:  0,
			mediaYear:   2008,
			tolerance:   1,
			isEpisode:   true,
			expectValid: true,
			expectScore: 30,
		},
		{
			name:        "episode: no year in media",
			parsedYear:  2008,
			mediaYear:   0,
			tolerance:   1,
			isEpisode:   true,
			expectValid: true,
			expectScore: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, score := validateYear(tt.parsedYear, tt.mediaYear, tt.tolerance, tt.isEpisode)
			if valid != tt.expectValid {
				t.Errorf("valid: got %v, want %v", valid, tt.expectValid)
			}
			if score != tt.expectScore {
				t.Errorf("score: got %v, want %v", score, tt.expectScore)
			}
		})
	}
}

func TestValidateSeasonEpisode(t *testing.T) {
	tests := []struct {
		name        string
		parsed      *ParsedNZB
		media       *domain.Media
		expectValid bool
		expectScore int
	}{
		{
			name: "movie (no season/episode)",
			parsed: &ParsedNZB{
				Season:  0,
				Episode: 0,
			},
			media: &domain.Media{
				Season: 0,
				Number: 0,
			},
			expectValid: true,
			expectScore: 20,
		},
		{
			name: "exact season and episode match",
			parsed: &ParsedNZB{
				Season:  1,
				Episode: 5,
			},
			media: &domain.Media{
				Season: 1,
				Number: 5,
			},
			expectValid: true,
			expectScore: 20,
		},
		{
			name: "season pack (episode 0)",
			parsed: &ParsedNZB{
				Season:  1,
				Episode: 0,
			},
			media: &domain.Media{
				Season: 1,
				Number: 5,
			},
			expectValid: true,
			expectScore: 10,
		},
		{
			name: "wrong season",
			parsed: &ParsedNZB{
				Season:  2,
				Episode: 5,
			},
			media: &domain.Media{
				Season: 1,
				Number: 5,
			},
			expectValid: false,
			expectScore: 0,
		},
		{
			name: "wrong episode",
			parsed: &ParsedNZB{
				Season:  1,
				Episode: 3,
			},
			media: &domain.Media{
				Season: 1,
				Number: 5,
			},
			expectValid: false,
			expectScore: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, score := validateSeasonEpisode(tt.parsed, tt.media)
			if valid != tt.expectValid {
				t.Errorf("valid: got %v, want %v", valid, tt.expectValid)
			}
			if score != tt.expectScore {
				t.Errorf("score: got %v, want %v", score, tt.expectScore)
			}
		})
	}
}

func TestScoreResolution(t *testing.T) {
	tests := []struct {
		name       string
		resolution string
		expected   int
	}{
		{"2160p", "2160P", 40},
		{"4K", "4K", 40},
		{"1080p", "1080P", 30},
		{"720p", "720P", 20},
		{"480p", "480P", 10},
		{"unknown", "", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreResolution(tt.resolution)
			if score != tt.expected {
				t.Errorf("got %v, want %v", score, tt.expected)
			}
		})
	}
}

func TestScoreSource(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected int
	}{
		{"REMUX", "REMUX", 30},
		{"BluRay", "BLURAY", 25},
		{"WEB-DL", "WEB-DL", 20},
		{"WEBRip", "WEBRIP", 15},
		{"HDTV", "HDTV", 10},
		{"unknown", "", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreSource(tt.source)
			if score != tt.expected {
				t.Errorf("got %v, want %v", score, tt.expected)
			}
		})
	}
}

func TestScoreCodec(t *testing.T) {
	tests := []struct {
		name     string
		codec    string
		expected int
	}{
		{"x265", "X265", 20},
		{"HEVC", "HEVC", 20},
		{"x264", "X264", 15},
		{"XviD", "XVID", 10},
		{"unknown", "", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scoreCodec(tt.codec)
			if score != tt.expected {
				t.Errorf("got %v, want %v", score, tt.expected)
			}
		})
	}
}

func TestValidateParsedNZB(t *testing.T) {
	cfg := &config.Config{
		TitleSimilarityMin: 0.7,
		YearTolerance:      1,
		MinValidationScore: 65,
	}

	tests := []struct {
		name        string
		parsed      *ParsedNZB
		media       *domain.Media
		expectValid bool
		minScore    int
	}{
		{
			name: "valid movie with high quality",
			parsed: &ParsedNZB{
				Title: "breaking bad",
				Year:  2008,
			},
			media: &domain.Media{
				Title:  "Breaking Bad",
				Year:   2008,
				Season: 0,
				Number: 0,
			},
			expectValid: true,
			minScore:    100,
		},
		{
			name: "valid episode exact match",
			parsed: &ParsedNZB{
				Title:   "breaking bad",
				Year:    2008,
				Season:  1,
				Episode: 1,
			},
			media: &domain.Media{
				Title:  "Breaking Bad",
				Year:   2008,
				Season: 1,
				Number: 1,
			},
			expectValid: true,
			minScore:    100,
		},
		{
			name: "invalid title similarity",
			parsed: &ParsedNZB{
				Title: "walking dead",
				Year:  2008,
			},
			media: &domain.Media{
				Title:  "Breaking Bad",
				Year:   2008,
				Season: 0,
				Number: 0,
			},
			expectValid: false,
			minScore:    0,
		},
		{
			name: "invalid year",
			parsed: &ParsedNZB{
				Title: "breaking bad",
				Year:  2020,
			},
			media: &domain.Media{
				Title:  "Breaking Bad",
				Year:   2008,
				Season: 0,
				Number: 0,
			},
			expectValid: false,
			minScore:    0,
		},
		{
			name: "invalid season",
			parsed: &ParsedNZB{
				Title:   "breaking bad",
				Year:    2008,
				Season:  2,
				Episode: 1,
			},
			media: &domain.Media{
				Title:  "Breaking Bad",
				Year:   2008,
				Season: 1,
				Number: 1,
			},
			expectValid: false,
			minScore:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, score := validateParsedNZB(tt.parsed, tt.media, cfg)
			if valid != tt.expectValid {
				t.Errorf("valid: got %v, want %v", valid, tt.expectValid)
			}
			if tt.expectValid && score < tt.minScore {
				t.Errorf("score: got %v, want at least %v", score, tt.minScore)
			}
		})
	}
}

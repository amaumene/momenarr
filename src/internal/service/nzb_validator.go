package service

import (
	"github.com/amaumene/momenarr/internal/config"
	"github.com/amaumene/momenarr/internal/domain"
)

const (
	maxTitleScore        = 50
	maxYearScore         = 30
	maxSeasonScore       = 10
	maxEpisodeScore      = 10
	yearExactMatchScore  = 30
	yearOneYearOffScore  = 20
	yearTwoYearsOffScore = 10
)

func validateParsedNZB(parsed *ParsedNZB, media *domain.Media, cfg *config.Config) (bool, int) {
	titleValid, titleScore := validateTitle(parsed.Title, media.Title, cfg.TitleSimilarityMin)
	if !titleValid {
		return false, 0
	}

	isEpisode := isMediaEpisode(media)
	yearValid, yearScore := validateYear(parsed.Year, media.Year, cfg.YearTolerance, isEpisode)
	if !yearValid {
		return false, 0
	}

	seValid, seScore := validateSeasonEpisode(parsed, media)
	if !seValid {
		return false, 0
	}

	totalScore := titleScore + yearScore + seScore
	return totalScore >= cfg.MinValidationScore, totalScore
}

func validateTitle(parsedTitle, mediaTitle string, minSimilarity float64) (bool, int) {
	normalizedMedia := normalizeTitle(mediaTitle)
	similarity := calculateSimilarity(parsedTitle, normalizedMedia)

	if similarity < minSimilarity {
		return false, 0
	}

	score := int(similarity * float64(maxTitleScore))
	return true, score
}

func calculateSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}

	distance := levenshteinDistance(a, b)
	maxLen := max(len(a), len(b))

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - (float64(distance) / float64(maxLen))
}

func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}

	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
		}
	}

	return matrix[len(a)][len(b)]
}

func validateYear(parsedYear, mediaYear, tolerance int64, isEpisode bool) (bool, int) {
	if mediaYear == 0 {
		return true, maxYearScore
	}

	if parsedYear == 0 {
		if isEpisode {
			return true, maxYearScore
		}
		return false, 0
	}

	diff := abs(parsedYear - mediaYear)

	if diff == 0 {
		return true, yearExactMatchScore
	}

	if isEpisode {
		return false, 0
	}

	if diff <= tolerance {
		return true, yearOneYearOffScore
	}
	if diff <= tolerance+1 {
		return true, yearTwoYearsOffScore
	}

	return false, 0
}

func validateSeasonEpisode(parsed *ParsedNZB, media *domain.Media) (bool, int) {
	if !isMediaEpisode(media) {
		return true, maxSeasonScore + maxEpisodeScore
	}

	if parsed.Season != media.Season {
		return false, 0
	}

	if parsed.Episode == 0 {
		return true, maxSeasonScore
	}

	if parsed.Episode == media.Number {
		return true, maxSeasonScore + maxEpisodeScore
	}

	return false, 0
}

func isMediaEpisode(media *domain.Media) bool {
	return media.Season > 0 && media.Number > 0
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func min(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	minVal := values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

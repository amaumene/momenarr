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

	// Levenshtein algorithm constants
	levenshteinSubstitutionCost = 1
	levenshteinInsertionCost    = 1
	levenshteinDeletionCost     = 1
	perfectSimilarityScore      = 1.0
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

func calculateSimilarity(str1, str2 string) float64 {
	if str1 == str2 {
		return perfectSimilarityScore
	}

	distance := levenshteinDistance(str1, str2)
	maxLen := max(len(str1), len(str2))

	if maxLen == 0 {
		return perfectSimilarityScore
	}

	return perfectSimilarityScore - (float64(distance) / float64(maxLen))
}

func levenshteinDistance(source, target string) int {
	if len(source) == 0 {
		return len(target)
	}
	if len(target) == 0 {
		return len(source)
	}

	// Ensure source is the shorter string for space optimization
	if len(source) > len(target) {
		source, target = target, source
	}

	// Use two rows instead of full matrix: O(min(n,m)) space instead of O(n×m)
	prevRow := make([]int, len(source)+1)
	currRow := make([]int, len(source)+1)

	for i := range prevRow {
		prevRow[i] = i
	}

	for j := 1; j <= len(target); j++ {
		currRow[0] = j
		for i := 1; i <= len(source); i++ {
			cost := levenshteinSubstitutionCost
			if source[i-1] == target[j-1] {
				cost = 0
			}
			currRow[i] = min(
				prevRow[i]+levenshteinDeletionCost,    // Deletion
				currRow[i-1]+levenshteinInsertionCost, // Insertion
				prevRow[i-1]+cost,                     // Substitution
			)
		}
		prevRow, currRow = currRow, prevRow
	}

	return prevRow[len(source)]
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

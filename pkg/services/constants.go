package services

import "regexp"

// Video file extensions
var VideoExtensions = []string{
	".mp4", ".mkv", ".avi", ".mov", ".wmv",
	".flv", ".webm", ".m4v", ".mpg", ".mpeg",
}

// Regular expressions
var (
	EpisodeRegex = regexp.MustCompile(`(?i)s(\d{2})e(\d{2})`)
)

// Batch processing constants
const (
	DefaultBatchSize = 200
	MaxBatchSize     = 500
)

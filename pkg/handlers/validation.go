package handlers

import (
	"errors"
	"strconv"
)

var (
	ErrInvalidTraktID    = errors.New("invalid trakt ID")
	ErrInvalidDownloadID = errors.New("invalid download ID")
)

// validateTraktID validates and parses a Trakt ID
func validateTraktID(traktIDStr string) (int64, error) {
	if traktIDStr == "" {
		return 0, ErrInvalidTraktID
	}
	
	traktID, err := strconv.ParseInt(traktIDStr, 10, 64)
	if err != nil {
		return 0, ErrInvalidTraktID
	}
	
	// Trakt IDs should be positive
	if traktID <= 0 {
		return 0, ErrInvalidTraktID
	}
	
	// Reasonable upper bound check
	if traktID > 999999999 {
		return 0, ErrInvalidTraktID
	}
	
	return traktID, nil
}

// validateDownloadID validates and parses a download ID
func validateDownloadID(downloadIDStr string) (int64, error) {
	if downloadIDStr == "" {
		return 0, ErrInvalidDownloadID
	}
	
	downloadID, err := strconv.ParseInt(downloadIDStr, 10, 64)
	if err != nil {
		return 0, ErrInvalidDownloadID
	}
	
	// Download IDs should be positive
	if downloadID <= 0 {
		return 0, ErrInvalidDownloadID
	}
	
	// Reasonable upper bound check
	if downloadID > 999999999 {
		return 0, ErrInvalidDownloadID
	}
	
	return downloadID, nil
}
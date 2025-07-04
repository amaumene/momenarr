package handlers

import (
	"errors"
	"strconv"
)

var (
	ErrInvalidTraktID = errors.New("invalid trakt ID")
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
package handlers

import (
	"errors"
	"strconv"
)

const maxTraktID = 999999999

var errInvalidTraktID = errors.New("invalid trakt ID")


func validateTraktID(traktIDStr string) (int64, error) {
	if traktIDStr == "" {
		return 0, errInvalidTraktID
	}

	traktID, err := strconv.ParseInt(traktIDStr, 10, 64)
	if err != nil {
		return 0, errInvalidTraktID
	}

	if traktID <= 0 || traktID > maxTraktID {
		return 0, errInvalidTraktID
	}

	return traktID, nil
}

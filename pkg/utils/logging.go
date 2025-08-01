// Package utils provides utility functions used across the application
package utils

import (
	"github.com/amaumene/momenarr/pkg/models"
	log "github.com/sirupsen/logrus"
)

// LogMediaOperation returns a log entry with common media fields
func LogMediaOperation(operation string, media *models.Media) *log.Entry {
	return log.WithFields(log.Fields{
		"trakt_id":  media.Trakt,
		"title":     media.Title,
		"type":      media.GetType(),
		"operation": operation,
	})
}

// LogTorrentSearchResultOperation returns a log entry with common torrent search result fields
func LogTorrentSearchResultOperation(operation string, result *models.TorrentSearchResult) *log.Entry {
	return log.WithFields(log.Fields{
		"hash":      result.Hash,
		"title":     result.Title,
		"source":    result.Source,
		"operation": operation,
	})
}

// LogDownloadOperation returns a log entry with download-related fields
func LogDownloadOperation(operation string, traktID int64, title string) *log.Entry {
	return log.WithFields(log.Fields{
		"trakt_id":  traktID,
		"title":     title,
		"operation": operation,
	})
}

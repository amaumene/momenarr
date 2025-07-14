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

// LogTorrentOperation returns a log entry with common torrent fields
func LogTorrentOperation(operation string, torrent *models.Torrent) *log.Entry {
	return log.WithFields(log.Fields{
		"hash":      torrent.Hash,
		"title":     torrent.Title,
		"trakt_id":  torrent.Trakt,
		"operation": operation,
	})
}

// LogDownloadOperation returns a log entry with download-related fields
func LogDownloadOperation(operation string, traktID int, title string) *log.Entry {
	return log.WithFields(log.Fields{
		"trakt_id":  traktID,
		"title":     title,
		"operation": operation,
	})
}

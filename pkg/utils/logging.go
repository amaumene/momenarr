
package utils

import (
	"github.com/amaumene/momenarr/pkg/models"
	log "github.com/sirupsen/logrus"
)


func LogMediaOperation(operation string, media *models.Media) *log.Entry {
	return log.WithFields(log.Fields{
		"trakt_id":  media.Trakt,
		"title":     media.Title,
		"type":      media.GetType(),
		"operation": operation,
	})
}


func LogTorrentSearchResultOperation(operation string, result *models.TorrentSearchResult) *log.Entry {
	return log.WithFields(log.Fields{
		"hash":      result.Hash,
		"title":     result.Title,
		"source":    result.Source,
		"operation": operation,
	})
}

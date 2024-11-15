package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"regexp"
)

func processNotification(notification torbox.Notification, appConfig App) {
	extractedString, err := extractString(notification.Data.Message)
	if err != nil {
		log.WithFields(log.Fields{
			"message": notification.Data.Message,
			"err":     err,
		}).Info("Extracting string")
		return
	}
	UsenetDownload, err := appConfig.torBoxClient.FindDownloadByName(extractedString)
	if err != nil {
		log.Printf("Error finding download: %v\n", err)
		log.WithFields(log.Fields{
			"string": extractedString,
			"err":    err,
		}).Info("Finding download")
		return
	}

	if notification.Data.Title == "Usenet Download Completed" {
		err = downloadFromTorBox(UsenetDownload, appConfig)
		if err != nil {
			log.WithFields(log.Fields{
				"id":   UsenetDownload[0].ID,
				"name": UsenetDownload[0].Name,
				"err":  err,
			}).Fatal("Downloading transfer")
		}
	}
	if notification.Data.Title == "Usenet Download Failed" {
		log.WithFields(log.Fields{
			"id":   UsenetDownload[0].ID,
			"name": UsenetDownload[0].Name,
			"err":  err,
		}).Warning("Usenet download failed")
		err = appConfig.torBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete")
		if err != nil {
			log.WithFields(log.Fields{
				"id":  UsenetDownload[0].ID,
				"err": err,
			}).Error("Deleting failed transfer")
		}
		err = appConfig.store.UpdateMatching(&NZB{}, bolthold.Where("Title").Eq(extractedString), func(record interface{}) error {
			update, ok := record.(*NZB) // record will always be a pointer
			if !ok {
				return fmt.Errorf("Record isn't the correct type!  Wanted NZB, got %T", record)
			}
			update.Failed = true
			return nil
		})
		var movies []Movie
		_ = appConfig.store.Find(&movies, bolthold.Where("DownloadID").Eq(UsenetDownload[0].ID))
		for _, movie := range movies {
			nzb, err := appConfig.getNzbFromDB(movie.IMDB)
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("Request NZB from database")
			} else {
				appConfig.createOrDownloadCachedMovie(movie.IMDB, nzb)
			}
		}
		var episodes []Episode
		_ = appConfig.store.Find(&episodes, bolthold.Where("DownloadID").Eq(UsenetDownload[0].ID))
		for _, episode := range episodes {
			nzb, err := appConfig.getNzbFromDB(episode.IMDB)
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("Request NZB from database")
			} else {
				appConfig.createOrDownloadCachedEpisode(episode.IMDB, nzb)
			}
		}
	}
	return
}

func extractString(message string) (string, error) {
	const regexPattern = `download (.+?) has`
	re := regexp.MustCompile(regexPattern)
	match := re.FindStringSubmatch(message)
	if len(match) < 2 {
		return "", fmt.Errorf("failed to extract the desired string")
	}
	return match[1], nil
}

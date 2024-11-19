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
		log.WithFields(log.Fields{
			"string": extractedString,
			"err":    err,
		}).Info("Finding download")
		return
	}

	switch notification.Data.Title {
	case "Usenet Download Completed":
		handleDownloadCompleted(appConfig, UsenetDownload)
	case "Usenet Download Failed":
		handleDownloadFailed(appConfig, UsenetDownload, extractedString)
	}
}

func handleDownloadCompleted(appConfig App, UsenetDownload []torbox.UsenetDownload) {
	var medias []Media
	err := appConfig.store.Find(&medias, bolthold.Where("DownloadID").Eq(UsenetDownload[0].ID))
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Finding media in database")
		return
	}

	for _, media := range medias {
		err = appConfig.downloadFromTorBox(UsenetDownload, media.IMDB)
		if err != nil {
			log.WithFields(log.Fields{
				"id":   UsenetDownload[0].ID,
				"name": UsenetDownload[0].Name,
				"err":  err,
			}).Error("Downloading transfer")
		}
	}
}

func handleDownloadFailed(appConfig App, UsenetDownload []torbox.UsenetDownload, extractedString string) {
	log.WithFields(log.Fields{
		"id":   UsenetDownload[0].ID,
		"name": UsenetDownload[0].Name,
	}).Warning("Usenet download failed")

	err := appConfig.torBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete")
	if err != nil {
		log.WithFields(log.Fields{
			"id":  UsenetDownload[0].ID,
			"err": err,
		}).Error("Deleting failed transfer")
	}

	err = appConfig.store.UpdateMatching(&NZB{}, bolthold.Where("Title").Eq(extractedString), func(record interface{}) error {
		update, ok := record.(*NZB)
		if !ok {
			return fmt.Errorf("record isn't the correct type! Wanted NZB, got %T", record)
		}
		update.Failed = true
		return nil
	})
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Updating NZB status in database")
	}

	var medias []Media
	err = appConfig.store.Find(&medias, bolthold.Where("DownloadID").Eq(UsenetDownload[0].ID))
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Finding media in database")
		return
	}

	for _, media := range medias {
		nzb, err := appConfig.getNzbFromDB(media.IMDB)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Requesting NZB from database")
		} else {
			appConfig.createOrDownloadCachedMedia(media.IMDB, nzb)
		}
	}
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

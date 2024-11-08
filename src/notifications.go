package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
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
	UsenetDownload, err := appConfig.TorBoxClient.FindDownloadByName(extractedString)
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
		}).Info("Usenet download failed")
		err = appConfig.TorBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete")
		if err != nil {
			log.WithFields(log.Fields{
				"id":  UsenetDownload[0].ID,
				"err": err,
			}).Fatal("Deleting failed transfer")
		}
		for i, current := range currentDownloads {
			for j, item := range current.Items {
				if item.Title == extractedString && item.Failed == false {
					currentDownloads[i].Items[j].Failed = true
					UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(currentDownloads[i].Items[j+1].Enclosure.URL, item.Title)
					if err != nil {
						log.WithFields(log.Fields{
							"name": current.Items[0].Title,
							"err":  err,
						}).Fatal("Creating transfer")
					}
					if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
						downloadCachedData(UsenetCreateDownloadResponse, appConfig)
					}
					break
				}
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

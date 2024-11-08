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
		log.Printf("Error extracting string: %v\n", err)
		return
	}
	UsenetDownload, err := appConfig.TorBoxClient.FindDownloadByName(extractedString)
	if err != nil {
		log.Printf("Error finding download: %v\n", err)
		return
	}

	if notification.Data.Title == "Usenet Download Completed" {
		downloadFromTorBox(UsenetDownload, appConfig)
	}
	if notification.Data.Title == "Usenet Download Failed" {
		fmt.Printf("Usenet Download Failed: %d\n", UsenetDownload[0].ID)
		err = appConfig.TorBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete")
		if err != nil {
			log.Printf("Error deleting download: %v\n", err)
			return
		}
		for i, movie := range currentMovies {
			for j, item := range movie.Items {
				if item.Title == extractedString && item.Failed == false {
					currentMovies[i].Items[j].Failed = true
					UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(item.Enclosure.URL, item.Title)
					if err != nil {
						log.WithFields(log.Fields{
							"item": movie.Items[0].Title,
							"err":  err,
						}).Fatal("Error creating transfer")
					}
					fmt.Println(UsenetCreateDownloadResponse)
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

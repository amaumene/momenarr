package main

import (
	"fmt"
	"github.com/amaumene/momenarr/internal/torbox"
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
	//if notification.Data.Title == "Usenet Download Failed" {
	//	log.WithFields(log.Fields{
	//		"id":   UsenetDownload[0].ID,
	//		"name": UsenetDownload[0].Name,
	//		"err":  err,
	//	}).Info("Usenet download failed")
	//	//err = appConfig.TorBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete")
	//	//if err != nil {
	//	//	log.WithFields(log.Fields{
	//	//		"id":  UsenetDownload[0].ID,
	//	//		"err": err,
	//	//	}).Fatal("Deleting failed transfer")
	//	//}
	//	movie := Movie{}
	//	err := appConfig.db.View(func(tx *buntdb.Tx) error {
	//		val, err := tx.Get(strconv.Itoa(UsenetDownload[0].ID))
	//		if err != nil {
	//			return err
	//		}
	//		err = json.Unmarshal([]byte(val), &movie)
	//		if err != nil {
	//			log.Fatal(err)
	//		}
	//		return nil
	//	})
	//	if err != nil {
	//		log.WithFields(log.Fields{
	//			"id":  UsenetDownload[0].ID,
	//			"err": err,
	//		}).Fatal("Couldn't get value from db")
	//	}
	//	for i, item := range movie.Item {
	//		if item.Title == extractedString && item.Failed == false {
	//			movie.Item[i].Failed = true
	//			log.WithFields(log.Fields{
	//				"name": movie.Item[i+1].Title,
	//			}).Info("Going to download")
	//			UsenetCreateDownloadResponse, err := appConfig.TorBoxClient.CreateUsenetDownload(movie.Item[i+1].Enclosure.Attributes.URL, movie.Item[i+1].Title)
	//			if err != nil {
	//				log.WithFields(log.Fields{
	//					"name": movie.Item[i+1].Title,
	//					"err":  err,
	//				}).Fatal("Creating transfer")
	//			}
	//			err = appConfig.db.Update(func(tx *buntdb.Tx) error {
	//				b, err := json.Marshal(movie)
	//				if err != nil {
	//					return err
	//				}
	//				_, _, err = tx.Set(strconv.Itoa(UsenetCreateDownloadResponse.Data.UsenetDownloadID), string(b), nil)
	//				return err
	//			})
	//			err = appConfig.db.Update(func(tx *buntdb.Tx) error {
	//				_, err = tx.Delete(strconv.Itoa(UsenetDownload[0].ID))
	//				return err
	//			})
	//			appConfig.db.Shrink()
	//			if UsenetCreateDownloadResponse.Detail == "Found cached usenet download. Using cached download." {
	//				downloadCachedData(UsenetCreateDownloadResponse, appConfig)
	//			}
	//			break
	//		}
	//	}
	//}
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

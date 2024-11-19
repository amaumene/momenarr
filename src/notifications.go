package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	"github.com/timshannon/bolthold"
	"regexp"
)

func processNotification(notification torbox.Notification, appConfig App) error {
	extractedString, err := extractString(notification.Data.Message)
	if err != nil {
		return fmt.Errorf("extracting string from notification: %v", err)
	}

	UsenetDownload, err := appConfig.torBoxClient.FindDownloadByName(extractedString)
	if err != nil {
		return fmt.Errorf("finding download by name: %v", err)
	}

	switch notification.Data.Title {
	case "Usenet Download Completed":
		err = handleDownloadCompleted(appConfig, UsenetDownload)
		if err != nil {
			return fmt.Errorf("handling download completed: %v", err)
		}
	case "Usenet Download Failed":
		err = handleDownloadFailed(appConfig, UsenetDownload, extractedString)
		if err != nil {
			return fmt.Errorf("handling download failed: %v", err)
		}
	}
	return nil
}

func handleDownloadCompleted(appConfig App, UsenetDownload []torbox.UsenetDownload) error {
	var medias []Media
	err := appConfig.store.Find(&medias, bolthold.Where("DownloadID").Eq(UsenetDownload[0].ID))
	if err != nil {
		return fmt.Errorf("finding media in database: %v", err)
	}

	for _, media := range medias {
		err = appConfig.downloadFromTorBox(UsenetDownload, media.IMDB)
		if err != nil {
			return fmt.Errorf("downloading media from TorBox: %v", err)
		}
	}
	return nil
}

func handleDownloadFailed(appConfig App, UsenetDownload []torbox.UsenetDownload, extractedString string) error {
	err := appConfig.torBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete")
	if err != nil {
		return fmt.Errorf("deleting transfer: %v", err)
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
		return fmt.Errorf("updating NZB record: %v", err)
	}

	var medias []Media
	err = appConfig.store.Find(&medias, bolthold.Where("DownloadID").Eq(UsenetDownload[0].ID))
	if err != nil {
		return fmt.Errorf("finding media in database: %v", err)
	}

	for _, media := range medias {
		nzb, err := appConfig.getNzbFromDB(media.IMDB)
		if err != nil {
			return fmt.Errorf("getting NZB from database: %v", err)
		} else {
			if err := appConfig.createOrDownloadCachedMedia(media.IMDB, nzb); err != nil {
				return fmt.Errorf("creating or downloading cached media: %v", err)
			}
		}
	}
	return nil
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

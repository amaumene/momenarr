package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	"log"
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
	downloadFromTorBox(UsenetDownload, appConfig)
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

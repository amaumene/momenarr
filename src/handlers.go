package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func handleAPIRequests(appConfig *App) {
	http.HandleFunc("/api/notify", func(w http.ResponseWriter, r *http.Request) {
		handlePostData(w, r, *appConfig)
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		go func() {
			appConfig.runTasks()
		}()
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Refresh initiated")); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("writing refresh response")
		}
	})
}

func handlePostData(w http.ResponseWriter, r *http.Request, appConfig App) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("failed to close request body")
		}
	}()

	var notification Notification
	err = json.Unmarshal(body, &notification)
	if err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
	go func() {
		err := processNotification(notification, appConfig)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("processing notification")
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"message": "Data received and processing started"}`)); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("writing response")
	}
}

func processNotification(notification Notification, appConfig App) error {
	if notification.Category == "momenarr" && notification.Status == "SUCCESS" {
		var media Media
		err := appConfig.store.Get(notification.IMDB, &media)
		if err != nil {
			return fmt.Errorf("finding media: %v", err)
		}
		file, err := findBiggestFile(notification.Dir)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Finding biggest file")
		}

		destPath := filepath.Join(appConfig.downloadDir, filepath.Base(file))
		err = os.Rename(file, destPath)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Moving file to download directory")
		}
		media.File = file
		media.OnDisk = true
		if err := appConfig.store.Update(notification.IMDB, &media); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Update media path/status in database")
		}

		IDs := []int64{
			media.downloadID,
		}
		result, err := appConfig.nzbget.EditQueue("HistoryFinalDelete", "", IDs)
		if err != nil || result == false {
			log.WithFields(log.Fields{"err": err}).Error("Failed to delete NZBGet download")
		}
	}
	return nil
}

// findBiggestFile finds the biggest file in the given directory and its subdirectories.
func findBiggestFile(dir string) (string, error) {
	var biggestFile string
	var maxSize int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Size() > maxSize {
			biggestFile = path
			maxSize = info.Size()
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return biggestFile, nil
}

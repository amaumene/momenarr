package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

func createDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("Failed to create directory %s: %v", dir, err)
	}
}

func cleanDir(tempDir string) {
	files, err := os.ReadDir(tempDir)
	if err != nil {
		log.Fatalf("Failed to read temp directory: %v", err)
	}

	for _, file := range files {
		if err := os.RemoveAll(filepath.Join(tempDir, file.Name())); err != nil {
			log.Printf("Failed to remove file %s: %v", file.Name(), err)
		}
	}
}

func findBiggestFile(downloads []torbox.UsenetDownload) ([]torbox.UsenetDownload, error) {
	for _, download := range downloads {
		largestFile := download.Files[0]
		for _, file := range download.Files {
			if file.Size > largestFile.Size {
				largestFile = file
			}
		}
		filteredDownload := []torbox.UsenetDownload{download}
		filteredDownload[0].Files = []torbox.UsenetDownloadFile{largestFile}
		return filteredDownload, nil
	}
	return nil, fmt.Errorf("cannot find biggest file in download")
}

func (appConfig *App) createOrDownloadCachedMedia(IMDB string, nzb NZB) error {
	torboxDownload, err := appConfig.torBoxClient.CreateUsenetDownload(nzb.Link, nzb.Title)
	if err != nil {
		log.WithFields(log.Fields{
			"title":  nzb.Title,
			"detail": torboxDownload.Detail,
			"err":    err,
		}).Error("Creating TorBox transfer")
	}
	if torboxDownload.Success {
		var media Media
		if err = appConfig.store.Get(IMDB, &media); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Failed to get media from database")
		}
		media.DownloadID = torboxDownload.Data.UsenetDownloadID
		if err = appConfig.store.Update(IMDB, &media); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Update media downloadID on database")
		}
		log.WithFields(log.Fields{
			"IMDB":  IMDB,
			"Title": nzb.Title,
		}).Info("Download started successfully")
	}
	if torboxDownload.Detail == "Found cached usenet download. Using cached download." {
		if err = appConfig.downloadCachedData(torboxDownload, IMDB); err != nil {
			log.WithFields(log.Fields{
				"movie": nzb.Title,
				"err":   err,
			}).Fatal("Error downloading cached data")
		}
	}
	return nil
}

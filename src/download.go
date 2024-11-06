package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/amaumene/momenarr/got"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
)

func compareMD5sum(appConfig App, UsenetDownload []torbox.UsenetDownload) (bool, error) {
	filePath := filepath.Join(appConfig.downloadDir, UsenetDownload[0].Files[0].ShortName)
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}

	md5sum := fmt.Sprintf("%x", hash.Sum(nil))
	if md5sum != UsenetDownload[0].Files[0].MD5 {
		return false, nil
	}
	return true, nil
}
func downloadWithGot(fileLink string, UsenetDownload []torbox.UsenetDownload, appConfig App) error {
	ctx := context.Background()
	//ctx, cancel := context.WithCancel(ctx)
	download := got.NewDownload(ctx, fileLink, filepath.Join(appConfig.downloadDir, UsenetDownload[0].Files[0].ShortName))
	download.Concurrency = 4
	download.ChunkSize = 1073741824 // 1GiB
	download.Interval = 10000       // 10 sec

	got.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15"

	if err := download.Init(); err != nil {
		return err
	}

	go func() {
		if err := download.Start(); err != nil {
			log.WithFields(log.Fields{
				"fileName": UsenetDownload[0].Files[0].ShortName,
			}).Fatal("Error when downloading")
		}
	}()

	download.RunProgress(func(d *got.Download) {
		log.WithFields(log.Fields{
			"fileName": UsenetDownload[0].Files[0].ShortName,
			"speed":    download.AvgSpeed() / 1024 / 1024,
			"size":     download.Size() / 1024 / 1024,
		}).Info("Downloading")
	})
	log.WithFields(log.Fields{
		"fileName": UsenetDownload[0].Files[0].ShortName,
	}).Info("Download finished")
	return nil
}

func downloadFromTorBox(UsenetDownload []torbox.UsenetDownload, appConfig App) error {
	biggestUsenetDownload, err := findBiggestFile(UsenetDownload)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"fileName": biggestUsenetDownload[0].Files[0].ShortName,
	}).Info("Starting download")

	fileLink, err := appConfig.TorBoxClient.RequestUsenetDownloadLink(biggestUsenetDownload)
	if err != nil {
		return err
	}

	err = downloadWithGot(fileLink, biggestUsenetDownload, appConfig)
	if err != nil {
		fileLink, err = appConfig.TorBoxClient.RequestUsenetDownloadLink(biggestUsenetDownload)
		if err != nil {
			return err
		}
		return downloadWithGot(fileLink, biggestUsenetDownload, appConfig)
	}

	downloadedMD5, err := compareMD5sum(appConfig, biggestUsenetDownload)
	if downloadedMD5 == false {
		fileLink, err = appConfig.TorBoxClient.RequestUsenetDownloadLink(biggestUsenetDownload)
		if err != nil {
			return err
		}
		return downloadWithGot(fileLink, biggestUsenetDownload, appConfig)
	}
	log.WithFields(log.Fields{
		"fileName": biggestUsenetDownload[0].Files[0].ShortName,
	}).Info("Download and md5sum check finished")
	return nil
}

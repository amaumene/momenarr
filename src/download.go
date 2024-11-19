package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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

func (appConfig App) downloadFromTorBox(UsenetDownload []torbox.UsenetDownload, IMDB string) error {
	biggestUsenetDownload, err := findBiggestFile(UsenetDownload)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"fileName": biggestUsenetDownload[0].Files[0].ShortName,
	}).Info("Starting download")

	fileLink, err := appConfig.torBoxClient.RequestUsenetDownloadLink(biggestUsenetDownload)
	if err != nil {
		return err
	}

	if err := downloadUsingHTTP(fileLink, biggestUsenetDownload, appConfig); err != nil {
		log.WithFields(log.Fields{
			"fileName": biggestUsenetDownload[0].Files[0].ShortName,
		}).Info("Download failed, trying again")
		return appConfig.downloadFromTorBox(UsenetDownload, IMDB)
	}

	downloadedFilePath := filepath.Join(appConfig.downloadDir, biggestUsenetDownload[0].Files[0].ShortName)
	fileInfo, err := os.Stat(downloadedFilePath)
	if err != nil {
		return err
	}

	if biggestUsenetDownload[0].Files[0].Size != fileInfo.Size() {
		log.WithFields(log.Fields{
			"fileName": biggestUsenetDownload[0].Files[0].ShortName,
			"size":     fileInfo.Size(),
			"expected": biggestUsenetDownload[0].Files[0].Size,
		}).Error("Check size failed, trying again")
		return appConfig.downloadFromTorBox(UsenetDownload, IMDB)
	}

	var media Media
	if err := appConfig.store.Get(IMDB, &media); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to get media from database")
		return err
	}

	media.File = biggestUsenetDownload[0].Files[0].ShortName
	media.OnDisk = true

	if err := appConfig.store.Update(IMDB, &media); err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to update media path/status in database")
		return err
	}
	if err := appConfig.torBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete"); err != nil {
		log.WithFields(log.Fields{
			"name":  UsenetDownload[0].Name,
			"error": err,
		}).Error("Failed to delete the usenet download")
	}
	return nil
}

func downloadUsingHTTP(fileLink string, usenetDownload []torbox.UsenetDownload, appConfig App) error {
	httpClient := &http.Client{}
	resp, err := httpClient.Get(fileLink)
	if err != nil {
		return fmt.Errorf("failed to download file content: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = fmt.Errorf("closing response body: %s", closeErr)
		}
	}()

	tempFile, err := os.CreateTemp(appConfig.tempDir, "tempfile-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer func() {
		if closeErr := tempFile.Close(); closeErr != nil {
			err = fmt.Errorf("closing temporary file: %s", closeErr)
		}
	}()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalDownloaded int64
	chunkSize := int64(1073741824) // 1GiB chunk

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := int64(i) * chunkSize
			end := start + chunkSize
			if i == 3 {
				end = usenetDownload[0].Files[0].Size
			}

			if err := fetchFileChunk(httpClient, resp.Request.URL.String(), start, end, tempFile, &mu, &totalDownloaded); err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("downloading chunk")
			}
		}(i)
	}

	wg.Wait()

	finalFilePath := filepath.Join(appConfig.downloadDir, usenetDownload[0].Files[0].ShortName)
	if err := os.Rename(tempFile.Name(), finalFilePath); err != nil {
		return err
	}
	return nil
}

func fetchFileChunk(httpClient *http.Client, url string, start, end int64, tempFile *os.File, mu *sync.Mutex, totalDownloaded *int64) error {
	const maxRetries = 300
	const retryDelay = 10 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := fetchChunkWithRetry(httpClient, url, start, end, tempFile, mu, totalDownloaded); err == nil {
			return nil
		} else if attempt < maxRetries {
			log.WithFields(log.Fields{
				"attempt":    attempt,
				"maxRetries": maxRetries,
				"retryDelay": retryDelay,
			}).Error("downloading chunk")
			time.Sleep(retryDelay)
		} else {
			return fmt.Errorf("error downloading chunk after %d attempts", maxRetries)
		}
	}
	return nil
}

func fetchChunkWithRetry(httpClient *http.Client, url string, start, end int64, tempFile *os.File, mu *sync.Mutex, totalDownloaded *int64) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
	partResp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error performing request: %v", err)
	}
	defer func() {
		if closeErr := partResp.Body.Close(); closeErr != nil {
			err = fmt.Errorf("closing response body: %s", closeErr)
		}
	}()

	if partResp.StatusCode < 200 || partResp.StatusCode >= 300 {
		return fmt.Errorf("error: received non-success status code %d", partResp.StatusCode)
	}

	partBuf := make([]byte, 32*1024)
	for {
		n, err := partResp.Body.Read(partBuf)
		if n > 0 {
			mu.Lock()
			if _, writeErr := tempFile.WriteAt(partBuf[:n], start); writeErr != nil {
				mu.Unlock()
				return fmt.Errorf("error writing to temporary file: %v", writeErr)
			}
			start += int64(n)
			*totalDownloaded += int64(n)
			mu.Unlock()
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading response body: %v", err)
		}
	}
	return nil
}

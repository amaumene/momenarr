package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func (appConfig App) downloadCachedData(UsenetCreateDownloadResponse torbox.UsenetCreateDownloadResponse, IMDB int64) error {
	log.WithFields(log.Fields{
		"id": UsenetCreateDownloadResponse.Data.UsenetDownloadID,
	}).Info("Found cached usenet download")
	UsenetDownload, err := appConfig.torBoxClient.FindDownloadByID(UsenetCreateDownloadResponse.Data.UsenetDownloadID)
	if err != nil {
		return err
	}
	if UsenetDownload[0].Cached {
		log.WithFields(log.Fields{
			"name": UsenetDownload[0].Name,
		}).Info("Starting download from cached data")

		// Start the downloadFromTorBox function in a new goroutine
		go func() {
			err := appConfig.downloadFromTorBox(UsenetDownload, IMDB)
			if err != nil {
				log.WithFields(log.Fields{
					"name":  UsenetDownload[0].Name,
					"error": err,
				}).Error("Failed to download from TorBox")
			} else {
				log.WithFields(log.Fields{
					"name": UsenetDownload[0].Name,
				}).Info("Download from TorBox complete")
				// Optionally, you can proceed with further actions like deleting the usenet download
				err = appConfig.torBoxClient.ControlUsenetDownload(UsenetDownload[0].ID, "delete")
				if err != nil {
					log.WithFields(log.Fields{
						"name":  UsenetDownload[0].Name,
						"error": err,
					}).Error("Failed to delete the usenet download")
				}
			}
		}()

		return nil
	}
	log.WithFields(log.Fields{
		"name": UsenetDownload[0].Name,
	}).Info("Not really in cache, skipping and hoping to get a notification")
	return nil
}

func (appConfig App) downloadFromTorBox(UsenetDownload []torbox.UsenetDownload, IMDB int64) error {
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

	err = downloadUsingHTTP(fileLink, biggestUsenetDownload, appConfig)
	if err != nil {
		log.WithFields(log.Fields{
			"fileName": biggestUsenetDownload[0].Files[0].ShortName,
		}).Info("Download failed, trying again")
		return appConfig.downloadFromTorBox(UsenetDownload, IMDB)
	}

	//downloadedMD5, err := compareMD5sum(appConfig, biggestUsenetDownload)
	//if downloadedMD5 == false {
	downloadedFilePath := filepath.Join(appConfig.downloadDir, biggestUsenetDownload[0].Files[0].ShortName)
	fileInfo, err := os.Stat(downloadedFilePath)
	if biggestUsenetDownload[0].Files[0].Size != fileInfo.Size() {
		log.WithFields(log.Fields{
			"fileName": biggestUsenetDownload[0].Files[0].ShortName,
			"size":     fileInfo.Size(),
			"expected": biggestUsenetDownload[0].Files[0].Size,
		}).Error("Check size failed, trying again")
		return appConfig.downloadFromTorBox(UsenetDownload, IMDB)
	} else {
		err = appConfig.store.UpdateMatching(&Media{}, bolthold.Where("IMDB").Eq(IMDB).Index("IMDB"), func(record interface{}) error {
			update, ok := record.(*Media) // record will always be a pointer
			if !ok {
				return fmt.Errorf("Record isn't the correct type!  Wanted media, got %T", record)
			}
			update.OnDisk = true
			update.File = biggestUsenetDownload[0].Files[0].ShortName
			return nil
		})
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("Update media path/status on database")
		}
		return nil
	}
}

func downloadUsingHTTP(fileLink string, usenetDownload []torbox.UsenetDownload, appConfig App) error {
	httpClient := &http.Client{}
	resp, err := httpClient.Get(fileLink)
	if err != nil {
		return fmt.Errorf("failed to download file content: %v", err)
	}
	defer resp.Body.Close()

	tempFile, err := os.CreateTemp(appConfig.tempDir, "tempfile-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer tempFile.Close()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalDownloaded int64
	chunkSize := int64(1073741824) // 1GiB chunk
	startTime := time.Now()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := int64(i) * chunkSize
			end := start + chunkSize
			if i == 3 {
				end = usenetDownload[0].Files[0].Size
			}

			if err := fetchFileChunk(httpClient, resp.Request.URL.String(), start, end, tempFile, &mu, totalDownloaded, startTime, usenetDownload[0].Files[0].ShortName, usenetDownload[0].Files[0].Size); err != nil {
				log.Println("Error downloading chunk:", err)
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

func fetchFileChunk(httpClient *http.Client, url string, start, end int64, tempFile *os.File, mu *sync.Mutex, totalDownloaded int64, startTime time.Time, shortName string, totalSize int64) error {
	const maxRetries = 300
	const retryDelay = 10 * time.Second

	var localErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		localErr = fetchChunkWithRetry(httpClient, url, start, end, tempFile, mu, &totalDownloaded, startTime, shortName, totalSize)
		if localErr == nil {
			return nil
		}

		if attempt < maxRetries {
			fmt.Printf("Error downloading chunk (attempt %d/%d): %v. Retrying in %v...\n", attempt, maxRetries, localErr, retryDelay)
			time.Sleep(retryDelay)
		} else {
			return fmt.Errorf("error downloading chunk after %d attempts: %v", maxRetries, localErr)
		}
	}
	return nil
}

func fetchChunkWithRetry(httpClient *http.Client, url string, start, end int64, tempFile *os.File, mu *sync.Mutex, totalDownloaded *int64, startTime time.Time, shortName string, totalSize int64) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
	partResp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error performing request: %v", err)
	}
	defer partResp.Body.Close()

	// Check for non-success status code
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
			// Print progress outside of the lock to reduce lock contention
			//elapsedTime := time.Since(startTime).Seconds()
			//speed := float64(*totalDownloaded) / elapsedTime / 1024 // speed in KB/s
			//fmt.Printf("\rDownloading %s... %.2f%% complete, Speed: %.2f KB/s", shortName, float64(*totalDownloaded)/float64(totalSize)*100, speed)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			// Retry the current chunk due to an error
			return fmt.Errorf("error reading response body: %v", err)
		}
	}
	return nil
}

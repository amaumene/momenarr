package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/melbahja/got"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func downloadFile(downloadURL string, file APIFile) error {
	resp, err := httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download file content: %v", err)
	}
	defer resp.Body.Close()

	writeContentToFile(resp, file)

	fmt.Printf("\nFile downloaded and saved as %s\n", file.ShortName)
	return nil
}

func writeContentToFile(resp *http.Response, file APIFile) error {
	tempFile, err := os.CreateTemp(tempDir, "tempfile-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer tempFile.Close()

	ctx := context.Background()
	got.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15"
	dl := got.NewDownload(ctx, resp.Request.URL.String(), tempFile.Name())
	// Init
	if err := dl.Init(); err != nil {
		fmt.Println(err)
	}
	// Start download
	if err := dl.Start(); err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Average speed was: %.2f MB/s\n", float64(dl.AvgSpeed())/1024/1024)

	// Verify if the downloaded file size matches the expected total size
	fileInfo, err := tempFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get temporary file info: %v", err)
	}
	if fileInfo.Size() != file.Size {
		log.Println("Downloaded file size does not match the expected size, restarting download...")
		time.Sleep(retryDelay)
		return downloadFile(resp.Request.URL.String(), file)
	}

	// Verify if the downloaded md5 sum matches the expected md5 sum
	tempFile.Seek(0, 0) // Reset the read pointer to the beginning of the file

	hasher := md5.New()
	if _, err := io.Copy(hasher, tempFile); err != nil {
		return fmt.Errorf("failed to compute md5 for the downloaded file: %v", err)
	}
	downloadedMd5 := fmt.Sprintf("%x", hasher.Sum(nil))

	if downloadedMd5 != file.Md5 {
		log.Println("Downloaded file MD5 does not match the expected MD5, restarting download...")
		time.Sleep(retryDelay)
		return downloadFile(resp.Request.URL.String(), file)
	}

	finalFilePath := filepath.Join(downloadDir, file.ShortName)
	if err := os.Rename(tempFile.Name(), finalFilePath); err != nil {
		return fmt.Errorf("failed to rename temporary file: %v", err)
	}

	return nil
}

func downloadChunk(url string, start, end int64, tempFile *os.File, mu *sync.Mutex) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
	req.Proto = "HTTP/2.0"
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

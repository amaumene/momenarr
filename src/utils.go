package momenarr

import (
	"fmt"
	"github.com/amaumene/momenarr/internal/torbox"
	"github.com/razsteinmetz/go-ptn"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// VideoInfo holds the extracted components of a video filename
type VideoInfo struct {
	Title        string
	Season       string
	Episode      string
	VideoQuality string
	AudioQuality string
	GroupName    string
}

func createDir(dir string) {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		log.Fatalf("Failed to create directory %s: %v", dir, err)
	}
}

func cleanDir(tempDir string) {
	files, err := os.ReadDir(tempDir)
	if err != nil {
		log.Fatalf("Failed to read temp directory: %v", err)
	}

	for _, file := range files {
		err := os.RemoveAll(filepath.Join(tempDir, file.Name()))
		if err != nil {
			log.Printf("Failed to remove file %s: %v", file.Name(), err)
		}
	}
}

// extractVideoInfo parses the filename and returns a VideoInfo struct
func extractVideoInfo(filename string) VideoInfo {
	parsed, err := ptn.Parse(filename)
	if err != nil {
		log.Fatalf("Failed to parse filename %s: %v", filename, err)
	}

	return VideoInfo{
		Title:        strings.ToLower(strings.TrimSpace(parsed.Title)),
		Season:       fmt.Sprintf("%02d", parsed.Season),
		Episode:      fmt.Sprintf("%02d", parsed.Episode),
		VideoQuality: strings.ToLower(strings.TrimSpace(parsed.Resolution)),
		AudioQuality: strings.ToLower(strings.TrimSpace(parsed.Audio)),
		GroupName:    strings.ToLower(strings.TrimSpace(parsed.Group)),
	}
}

// compareVideos compares two video filenames for equivalence
func compareVideos(file1, file2 string) bool {
	info1 := extractVideoInfo(file1)
	info2 := extractVideoInfo(file2)

	return info1.Title == info2.Title && info1.Season == info2.Season &&
		info1.Episode == info2.Episode && info1.VideoQuality == info2.VideoQuality &&
		info1.AudioQuality == info2.AudioQuality && info1.GroupName == info2.GroupName
}

func fileExists(filename string, downloadDir string) (bool, error) {
	{
		files, err := os.ReadDir(downloadDir)
		if err != nil {
			return false, fmt.Errorf("failed to read download directory: %v", err)
		}

		for _, file := range files {
			fileNameWithoutExt := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
			//if compareVideos(fileNameWithoutExt, filename) {
			//	return true, nil
			//}
			if fileNameWithoutExt == filename {
				return true, nil
			}
		}
		return false, nil
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
		filteredDownload := []torbox.UsenetDownload{}

		filteredDownload = append(filteredDownload, download)
		filteredDownload[0].Files = []torbox.UsenetDownloadFile{largestFile}
		return filteredDownload, nil
	}
	return nil, fmt.Errorf("cannot find biggest file in download")
}

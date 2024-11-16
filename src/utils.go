package main

import (
	"fmt"
	"github.com/amaumene/momenarr/torbox"
	"log"
	"os"
	"path/filepath"
)

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

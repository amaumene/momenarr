package main

import (
	"fmt"
	tnp "github.com/ProfChaos/torrent-name-parser"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// compareFilenames checks if two filenames belong to the same episode.
func compareFilenames(filename1 string, filename2 string) (bool, error) {
	// Parse the first filename
	parsed1, err := tnp.ParseName(filename1)
	if err != nil {
		return false, fmt.Errorf("failed to parse first filename: %v", err)
	}

	// Parse the second filename
	parsed2, err := tnp.ParseName(filename2)
	if err != nil {
		return false, fmt.Errorf("failed to parse second filename: %v", err)
	}

	compareResult := map[string]bool{}

	// Use reflection to iterate through the fields of the Torrent struct
	t1Value := reflect.ValueOf(parsed1)
	t2Value := reflect.ValueOf(parsed2)
	tType := t1Value.Type()

	for i := 0; i < t1Value.NumField(); i++ {
		fieldName := tType.Field(i).Name
		if fieldName == "Container" {
			continue
		}
		v1 := t1Value.Field(i).Interface()
		v2 := t2Value.Field(i).Interface()
		if fieldName == "Group" {
			v1Str, ok1 := v1.(string)
			v2Str, ok2 := v2.(string)
			if ok1 && ok2 {
				v1 = strings.ToLower(v1Str)
				v2 = strings.ToLower(v2Str)
			}
		}
		match := reflect.DeepEqual(v1, v2)
		compareResult[fieldName] = match
		fmt.Printf("%s: %v vs %v -> Match: %v\n", fieldName, v1, v2, match)
	}

	// Determine final result
	allMatch := true
	for _, match := range compareResult {
		if !match {
			allMatch = false
			break
		}
	}

	fmt.Printf("Overall Match: %v\n", allMatch)
	return allMatch, nil
}

func fileExists(filename string) (bool, error) {
	files, err := os.ReadDir(downloadDir)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		matches, err := compareFilenames(filename, file.Name())
		if err != nil {
			fmt.Printf("Error comparing filenames: %v\n", err)
			return false, nil
		}
		if matches {
			return true, nil
		}
	}
	return false, nil
}

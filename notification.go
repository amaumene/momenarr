package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"time"
)

// Notification represents the structure of a notification with JSON tags for serialization.
type Notification struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      Data      `json:"data"`
}

// Data holds the notification content details such as title and message.
type Data struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

// processNotification handles an incoming notification by extracting data, performing API requests, and initiating a download.
func processNotification(notification Notification) {
	// Attempt to extract the string from the notification message
	extractedString, err := extractString(notification.Data.Message)
	if err != nil {
		log.Printf("Error extracting string: %v\n", err)
		return
	}

	// Perform a GET request to an API and retrieve the response body
	respBody, err := performGetRequest(apiURL, torboxApiKey)
	if err != nil {
		log.Printf("API request error: %v\n", err)
		return
	}

	// Parse the API response
	var apiResponse APIResponse
	if err := json.Unmarshal(respBody, &apiResponse); err != nil {
		log.Printf("Failed to parse API response: %v\n", err)
		return
	}

	// Find the matching item in the API response using the extracted string
	itemID, file, err := findMatchingItemByName(apiResponse, extractedString)
	if err != nil {
		log.Printf("Error finding matching item: %v\n", err)
		return
	}

	// Request to download the found item
	if err := requestDownload(itemID, file, torboxApiKey); err != nil {
		log.Printf("Download request error: %v\n", err)
	}
}

// extractString uses a regular expression to extract a specific substring from the message.
func extractString(message string) (string, error) {
	const regexPattern = `download (.+?) has`
	re := regexp.MustCompile(regexPattern)
	match := re.FindStringSubmatch(message)
	if len(match) < 2 {
		return "", fmt.Errorf("failed to extract the desired string")
	}
	return match[1], nil
}

package main

import (
	"encoding/json"
	"fmt"
	"github.com/jacklaaa89/trakt"
	"github.com/jacklaaa89/trakt/authorization"
	"log"
	"os"
)

func getToken(clientSecret string, tokenFile string) (*trakt.Token, error) {
	if _, err := os.Stat(tokenFile); err == nil {
		return loadTokenFromFile(tokenFile)
	}
	return generateNewToken(clientSecret, tokenFile)
}

func loadTokenFromFile(tokenFile string) (*trakt.Token, error) {
	file, err := os.Open(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("error opening token file: %v", err)
	}
	defer file.Close()

	var token trakt.Token
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&token); err != nil {
		return nil, fmt.Errorf("error decoding token from JSON: %v", err)
	}
	return &token, nil
}

func generateNewToken(clientSecret string, tokenFile string) (*trakt.Token, error) {
	deviceCode, err := authorization.NewCode(nil)
	if err != nil {
		return nil, fmt.Errorf("error generating device code: %v", err)
	}

	fmt.Printf("Please go to %s and enter the code: %s\n", deviceCode.VerificationURL, deviceCode.UserCode)

	pollParams := &trakt.PollCodeParams{
		Code:         deviceCode.Code,
		Interval:     deviceCode.Interval,
		ExpiresIn:    deviceCode.ExpiresIn,
		ClientSecret: clientSecret,
	}
	token, err := authorization.Poll(pollParams)
	if err != nil {
		return nil, fmt.Errorf("error polling for token: %v", err)
	}

	if err := saveTokenToFile(token, tokenFile); err != nil {
		return nil, err
	}
	return token, nil
}

func saveTokenToFile(token *trakt.Token, tokenFile string) error {
	file, err := os.Create(tokenFile)
	if err != nil {
		return fmt.Errorf("error creating token file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(token); err != nil {
		return fmt.Errorf("error encoding token to JSON: %v", err)
	}
	return nil
}

func setUpTrakt() *trakt.Token {
	trakt.Key = traktApiKey
	clientSecret := traktClientSecret

	if trakt.Key == "" || clientSecret == "" {
		log.Fatalf("TRAKT_API_KEY and TRAKT_CLIENT_SECRET must be set in environment variables")
	}

	tokenPath := os.Getenv("TOKEN_PATH")
	if tokenPath == "" {
		log.Printf("TOKEN_PATH not set, using current directory")
		tokenPath = "."
	}
	tokenFile := tokenPath + "/token.json"

	token, err := getToken(clientSecret, tokenFile)
	if err != nil {
		log.Fatalf("Error getting token: %v", err)
	}
	return token
}

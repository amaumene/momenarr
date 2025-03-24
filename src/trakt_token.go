package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/authorization"
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
	defer func() {
		if err := file.Close(); err != nil {
			err = fmt.Errorf("error closing the file: %v", err)
		}
	}()

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
	defer func() {
		if err := file.Close(); err != nil {
			err = fmt.Errorf("error closing the file: %v", err)
		}
	}()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(token); err != nil {
		return fmt.Errorf("error encoding token to JSON: %v", err)
	}
	return nil
}

func (app App) setUpTrakt(traktApiKey string, traktClientSecret string) *trakt.Token {
	trakt.Key = traktApiKey

	tokenFile := app.Config.DataDir + "/token.json"

	token, err := getToken(traktClientSecret, tokenFile)
	if err != nil {
		log.Fatalf("Error getting token: %v", err)
	}
	return token
}

func (app App) refreshTraktToken(traktClientSecret string) *trakt.Token {
	tokenFile := app.Config.DataDir + "/token.json"
	tokenFromFile, _ := loadTokenFromFile(tokenFile)
	refreshedToken, err := authorization.RefreshToken(&trakt.RefreshTokenParams{
		RefreshToken: tokenFromFile.RefreshToken,
		ClientSecret: traktClientSecret,
	})
	if err != nil {
		log.Fatalf("Error refreshing token: %v", err)
	}
	if err := saveTokenToFile(refreshedToken, tokenFile); err != nil {
		log.Fatalf("Error saving token: %v", err)
	}
	return refreshedToken
}

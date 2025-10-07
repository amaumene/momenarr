package main

import (
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/authorization"
	"log"
	"os"
)

const (
	tokenFileName = "/token.json"
)

func getToken(clientSecret string, tokenFile string) (*trakt.Token, error) {
	if tokenFileExists(tokenFile) {
		return loadTokenFromFile(tokenFile)
	}
	return generateNewToken(clientSecret, tokenFile)
}

func tokenFileExists(tokenFile string) bool {
	_, err := os.Stat(tokenFile)
	return err == nil
}

func loadTokenFromFile(tokenFile string) (*trakt.Token, error) {
	file, err := openTokenFile(tokenFile)
	if err != nil {
		return nil, err
	}
	defer closeTokenFile(file)

	return decodeTokenFromFile(file)
}

func openTokenFile(tokenFile string) (*os.File, error) {
	file, err := os.Open(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("error opening token file: %w", err)
	}
	return file, nil
}

func closeTokenFile(file *os.File) {
	if err := file.Close(); err != nil {
		log.Printf("error closing the file: %v", err)
	}
}

func decodeTokenFromFile(file *os.File) (*trakt.Token, error) {
	var token trakt.Token
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&token); err != nil {
		return nil, fmt.Errorf("error decoding token from JSON: %w", err)
	}
	return &token, nil
}

func generateNewToken(clientSecret string, tokenFile string) (*trakt.Token, error) {
	deviceCode, err := getDeviceCode()
	if err != nil {
		return nil, err
	}

	displayAuthInstructions(deviceCode)

	token, err := pollForToken(deviceCode, clientSecret)
	if err != nil {
		return nil, err
	}

	if err := saveTokenToFile(token, tokenFile); err != nil {
		return nil, err
	}
	return token, nil
}

func getDeviceCode() (*trakt.DeviceCode, error) {
	deviceCode, err := authorization.NewCode(nil)
	if err != nil {
		return nil, fmt.Errorf("error generating device code: %w", err)
	}
	return deviceCode, nil
}

func displayAuthInstructions(deviceCode *trakt.DeviceCode) {
	fmt.Printf("Please go to %s and enter the code: %s\n", deviceCode.VerificationURL, deviceCode.UserCode)
}

func pollForToken(deviceCode *trakt.DeviceCode, clientSecret string) (*trakt.Token, error) {
	pollParams := buildPollParams(deviceCode, clientSecret)
	token, err := authorization.Poll(pollParams)
	if err != nil {
		return nil, fmt.Errorf("error polling for token: %w", err)
	}
	return token, nil
}

func buildPollParams(deviceCode *trakt.DeviceCode, clientSecret string) *trakt.PollCodeParams {
	return &trakt.PollCodeParams{
		Code:         deviceCode.Code,
		Interval:     deviceCode.Interval,
		ExpiresIn:    deviceCode.ExpiresIn,
		ClientSecret: clientSecret,
	}
}

func saveTokenToFile(token *trakt.Token, tokenFile string) error {
	file, err := createTokenFile(tokenFile)
	if err != nil {
		return err
	}
	defer closeTokenFile(file)

	return encodeTokenToFile(token, file)
}

func createTokenFile(tokenFile string) (*os.File, error) {
	file, err := os.Create(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("error creating token file: %w", err)
	}
	return file, nil
}

func encodeTokenToFile(token *trakt.Token, file *os.File) error {
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(token); err != nil {
		return fmt.Errorf("error encoding token to JSON: %w", err)
	}
	return nil
}

func (app App) setUpTrakt(traktApiKey string, traktClientSecret string) *trakt.Token {
	trakt.Key = traktApiKey
	tokenFile := app.buildTokenFilePath()

	token, err := getToken(traktClientSecret, tokenFile)
	if err != nil {
		log.Fatalf("Error getting token: %v", err)
	}
	return token
}

func (app App) buildTokenFilePath() string {
	return app.Config.DataDir + tokenFileName
}

func (app App) refreshTraktToken(traktClientSecret string) *trakt.Token {
	tokenFile := app.buildTokenFilePath()

	tokenFromFile, _ := loadTokenFromFile(tokenFile)

	refreshedToken, err := refreshToken(tokenFromFile, traktClientSecret)
	if err != nil {
		log.Fatalf("Error refreshing token: %v", err)
	}

	if err := saveTokenToFile(refreshedToken, tokenFile); err != nil {
		log.Fatalf("Error saving token: %v", err)
	}
	return refreshedToken
}

func refreshToken(tokenFromFile *trakt.Token, clientSecret string) (*trakt.Token, error) {
	params := buildRefreshParams(tokenFromFile, clientSecret)
	refreshedToken, err := authorization.RefreshToken(params)
	if err != nil {
		return nil, err
	}
	return refreshedToken, nil
}

func buildRefreshParams(token *trakt.Token, clientSecret string) *trakt.RefreshTokenParams {
	return &trakt.RefreshTokenParams{
		RefreshToken: token.RefreshToken,
		ClientSecret: clientSecret,
	}
}

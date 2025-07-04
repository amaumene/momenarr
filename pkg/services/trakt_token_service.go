package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/authorization"
	log "github.com/sirupsen/logrus"
)

// TraktTokenService handles Trakt token management
type TraktTokenService struct {
	tokenFile    string
	clientSecret string
}

// NewTraktTokenService creates a new TraktTokenService
func NewTraktTokenService(dataDir, clientSecret string) *TraktTokenService {
	return &TraktTokenService{
		tokenFile:    filepath.Join(dataDir, "token.json"),
		clientSecret: clientSecret,
	}
}

// GetToken gets a valid Trakt token, creating one if necessary
func (s *TraktTokenService) GetToken() (*trakt.Token, error) {
	if s.tokenExists() {
		return s.loadTokenFromFile()
	}
	return s.generateNewToken()
}

// RefreshToken refreshes an existing Trakt token
func (s *TraktTokenService) RefreshToken(currentToken *trakt.Token) (*trakt.Token, error) {
	refreshedToken, err := authorization.RefreshToken(&trakt.RefreshTokenParams{
		RefreshToken: currentToken.RefreshToken,
		ClientSecret: s.clientSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("refreshing Trakt token: %w", err)
	}

	if err := s.saveTokenToFile(refreshedToken); err != nil {
		return nil, fmt.Errorf("saving refreshed token: %w", err)
	}

	log.Info("Successfully refreshed Trakt token")
	return refreshedToken, nil
}

// tokenExists checks if the token file exists
func (s *TraktTokenService) tokenExists() bool {
	_, err := os.Stat(s.tokenFile)
	return err == nil
}

// loadTokenFromFile loads a token from the token file
func (s *TraktTokenService) loadTokenFromFile() (*trakt.Token, error) {
	file, err := os.Open(s.tokenFile)
	if err != nil {
		return nil, fmt.Errorf("opening token file %s: %w", s.tokenFile, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.WithError(closeErr).Error("Failed to close token file")
		}
	}()

	var token trakt.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding token from JSON: %w", err)
	}

	log.WithField("token_file", s.tokenFile).Debug("Successfully loaded token from file")
	return &token, nil
}

// generateNewToken generates a new token using device authorization
func (s *TraktTokenService) generateNewToken() (*trakt.Token, error) {
	deviceCode, err := authorization.NewCode(nil)
	if err != nil {
		return nil, fmt.Errorf("generating device code: %w", err)
	}

	log.WithFields(log.Fields{
		"verification_url": deviceCode.VerificationURL,
		"user_code":        deviceCode.UserCode,
		"expires_in":       deviceCode.ExpiresIn,
	}).Info("Please authorize the application")

	fmt.Printf("Please go to %s and enter the code: %s\n", deviceCode.VerificationURL, deviceCode.UserCode)

	pollParams := &trakt.PollCodeParams{
		Code:         deviceCode.Code,
		Interval:     deviceCode.Interval,
		ExpiresIn:    deviceCode.ExpiresIn,
		ClientSecret: s.clientSecret,
	}

	token, err := authorization.Poll(pollParams)
	if err != nil {
		return nil, fmt.Errorf("polling for token: %w", err)
	}

	if err := s.saveTokenToFile(token); err != nil {
		return nil, fmt.Errorf("saving new token: %w", err)
	}

	log.Info("Successfully generated and saved new Trakt token")
	return token, nil
}

// saveTokenToFile saves a token to the token file
func (s *TraktTokenService) saveTokenToFile(token *trakt.Token) error {
	// Ensure directory exists
	dir := filepath.Dir(s.tokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating token directory %s: %w", dir, err)
	}

	file, err := os.OpenFile(s.tokenFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating token file %s: %w", s.tokenFile, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.WithError(closeErr).Error("Failed to close token file")
		}
	}()

	if err := json.NewEncoder(file).Encode(token); err != nil {
		return fmt.Errorf("encoding token to JSON: %w", err)
	}

	log.WithField("token_file", s.tokenFile).Debug("Successfully saved token to file")
	return nil
}

// ValidateToken validates that a token is still valid
func (s *TraktTokenService) ValidateToken(token *trakt.Token) bool {
	if token == nil {
		return false
	}

	if token.AccessToken == "" || token.RefreshToken == "" {
		return false
	}

	// Additional validation could be added here, such as:
	// - Checking token expiration
	// - Making a test API call

	return true
}

package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/authorization"
	log "github.com/sirupsen/logrus"
)

type TraktClient struct {
	token        *trakt.Token
	clientSecret string
	tokenPath    string
}

func NewTraktClient(apiKey, clientSecret, tokenPath string) (*TraktClient, error) {
	trakt.Key = apiKey

	configureRetryLogic()

	token, err := loadOrGenerateToken(clientSecret, tokenPath)
	if err != nil {
		return nil, fmt.Errorf("loading trakt token: %w", err)
	}

	return &TraktClient{
		token:        token,
		clientSecret: clientSecret,
		tokenPath:    tokenPath,
	}, nil
}

func configureRetryLogic() {
	trakt.WithConfig(&trakt.BackendConfig{
		MaxNetworkRetries: 3,
		HTTPClient:        &http.Client{Timeout: 80 * time.Second},
	})
}

func (c *TraktClient) Token() *trakt.Token {
	return c.token
}

func (c *TraktClient) RefreshToken(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	params := &trakt.RefreshTokenParams{
		RefreshToken: c.token.RefreshToken,
		ClientSecret: c.clientSecret,
	}

	refreshedToken, err := authorization.RefreshToken(params)
	if err != nil {
		return fmt.Errorf("refreshing token: %w", err)
	}

	if err := saveToken(refreshedToken, c.tokenPath); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	c.token = refreshedToken
	return nil
}

func loadOrGenerateToken(clientSecret, tokenPath string) (*trakt.Token, error) {
	if fileExists(tokenPath) {
		return loadToken(tokenPath)
	}
	return generateToken(clientSecret, tokenPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func loadToken(tokenPath string) (*trakt.Token, error) {
	file, err := os.Open(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("opening token file: %w", err)
	}
	defer file.Close()

	var token trakt.Token
	if err := json.NewDecoder(file).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}
	return &token, nil
}

func generateToken(clientSecret, tokenPath string) (*trakt.Token, error) {
	deviceCode, err := authorization.NewCode(nil)
	if err != nil {
		return nil, fmt.Errorf("generating device code: %w", err)
	}

	fmt.Printf("Please go to %s and enter the code: %s\n", deviceCode.VerificationURL, deviceCode.UserCode)

	token, err := pollForToken(deviceCode, clientSecret)
	if err != nil {
		return nil, fmt.Errorf("polling for token: %w", err)
	}

	if err := saveToken(token, tokenPath); err != nil {
		return nil, fmt.Errorf("saving token: %w", err)
	}
	return token, nil
}

func pollForToken(deviceCode *trakt.DeviceCode, clientSecret string) (*trakt.Token, error) {
	params := &trakt.PollCodeParams{
		Code:         deviceCode.Code,
		Interval:     deviceCode.Interval,
		ExpiresIn:    deviceCode.ExpiresIn,
		ClientSecret: clientSecret,
	}

	token, err := authorization.Poll(params)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func saveToken(token *trakt.Token, tokenPath string) error {
	file, err := os.Create(tokenPath)
	if err != nil {
		return fmt.Errorf("creating token file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(token); err != nil {
		return fmt.Errorf("encoding token: %w", err)
	}
	return nil
}

func (c *TraktClient) RefreshPeriodically(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.RefreshToken(ctx); err != nil {
				c.logRefreshError(err)
			}
		}
	}
}

func (c *TraktClient) logRefreshError(err error) {
	if isTransientError(err) {
		log.WithField("error", err).Warn("transient error refreshing trakt token, will retry next cycle")
	} else {
		log.WithField("error", err).Error("failed to refresh trakt token")
	}
}

func isTransientError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "502") || strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") || strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection")
}

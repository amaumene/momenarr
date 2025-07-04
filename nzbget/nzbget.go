package nzbget

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/amaumene/momenarr/internal/jsonrpc"
)

// Package defaults.
const (
	DefaultTimeout = 1 * time.Minute
)

// Config is the input data needed to return a NZBGet struct.
// This is setup to allow you to easily pass this data in from a config file.
type Config struct {
	URL    string       `json:"url" toml:"url" xml:"url" yaml:"url"`
	User   string       `json:"user" toml:"user" xml:"user" yaml:"user"`
	Pass   string       `json:"pass" toml:"pass" xml:"pass" yaml:"pass"`
	Client *http.Client `json:"-" toml:"-" xml:"-" yaml:"-"` // optional.
}

// NZBGet is what you get in return for passing in a valid Config to New().
type NZBGet struct {
	client *client
	url    string
}

type client struct {
	Auth string
	*http.Client
}

func New(config *Config) *NZBGet {
	// Set username and password if one's configured.
	auth := config.User + ":" + config.Pass
	if auth != ":" {
		auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	} else {
		auth = ""
	}

	httpClient := config.Client
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &NZBGet{
		url: strings.TrimSuffix(strings.TrimSuffix(config.URL, "/"), "/jsonrpc") + "/jsonrpc",
		client: &client{
			Auth:   auth,
			Client: httpClient,
		},
	}
}

// GetInto is a helper method to make a JSON-RPC request and turn the response into structured data.
func (n *NZBGet) GetInto(ctx context.Context, method string, output interface{}, args ...interface{}) error {
	message, err := jsonrpc.EncodeClientRequest(method, args)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewBuffer(message))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if n.client.Auth != "" {
		req.Header.Set("Authorization", n.client.Auth)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if err := jsonrpc.DecodeClientResponse(resp.Body, &output); err != nil {
		return fmt.Errorf("parsing response: %w: %s", err, resp.Status)
	}

	return nil
}

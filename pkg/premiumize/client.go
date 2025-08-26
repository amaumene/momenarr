package premiumize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

const (
	baseURL        = "https://www.premiumize.me/api"
	defaultTimeout = 30 * time.Second
)

type Config struct {
	APIKey string
	Client *http.Client
}

type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

func NewClient(config *Config) (*Client, error) {
	if config.APIKey == "" {
		return nil, ErrAPIKeyNotSet
	}

	httpClient := config.Client
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultTimeout,
		}
	}

	return &Client{
		apiKey:     config.APIKey,
		httpClient: httpClient,
		baseURL:    baseURL,
	}, nil
}

func (c *Client) buildURL(endpoint string) string {
	return fmt.Sprintf("%s%s?apikey=%s", c.baseURL, endpoint, c.apiKey)
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(endpoint), body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if body != nil && method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}

func (c *Client) doJSONRequest(ctx context.Context, method, endpoint string, body io.Reader, result interface{}) error {
	resp, err := c.doRequest(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

func (c *Client) CreateTransfer(ctx context.Context, nzbData []byte, folderID string) (*Transfer, error) {
	return c.CreateTransferWithFilename(ctx, nzbData, "download.nzb", folderID)
}

func (c *Client) CreateTransferWithFilename(ctx context.Context, nzbData []byte, filename string, folderID string) (*Transfer, error) {
	body, contentType, err := c.prepareTransferRequest(nzbData, filename, folderID)
	if err != nil {
		return nil, err
	}

	result, err := c.executeTransferRequest(ctx, body, contentType)
	if err != nil {
		return nil, err
	}

	return c.parseTransferResponse(result)
}

func (c *Client) prepareTransferRequest(nzbData []byte, filename string, folderID string) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("src", filename)
	if err != nil {
		return nil, "", fmt.Errorf("creating form file: %w", err)
	}

	if _, err := part.Write(nzbData); err != nil {
		return nil, "", fmt.Errorf("writing NZB data: %w", err)
	}

	if folderID != "" {
		if err := writer.WriteField("folder_id", folderID); err != nil {
			return nil, "", fmt.Errorf("writing folder_id field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("closing multipart writer: %w", err)
	}

	return &buf, writer.FormDataContentType(), nil
}

func (c *Client) executeTransferRequest(ctx context.Context, body *bytes.Buffer, contentType string) (*TransferCreateResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.buildURL("/transfer/create"), body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result TransferCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

func (c *Client) parseTransferResponse(result *TransferCreateResponse) (*Transfer, error) {
	if result.Status != "success" {
		return nil, fmt.Errorf("transfer creation failed: %s", result.Message)
	}

	return &Transfer{
		ID:     result.ID,
		Name:   result.Name,
		Status: TransferStatusQueued,
	}, nil
}

func (c *Client) GetTransfers(ctx context.Context) ([]Transfer, error) {
	var result TransferListResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, "/transfer/list", nil, &result); err != nil {
		return nil, fmt.Errorf("getting transfers: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("failed to get transfers: %s", result.Message)
	}

	return result.Transfers, nil
}

func (c *Client) GetTransfer(ctx context.Context, transferID string) (*Transfer, error) {
	transfers, err := c.GetTransfers(ctx)
	if err != nil {
		return nil, err
	}

	for _, transfer := range transfers {
		if transfer.ID == transferID {
			return &transfer, nil
		}
	}

	return nil, ErrTransferNotFound
}

func (c *Client) DeleteTransfer(ctx context.Context, transferID string) error {
	values := url.Values{}
	values.Set("id", transferID)

	var result BaseResponse
	if err := c.doJSONRequest(ctx, http.MethodPost, "/transfer/delete", bytes.NewBufferString(values.Encode()), &result); err != nil {
		return fmt.Errorf("deleting transfer: %w", err)
	}

	if result.Status != "success" {
		return fmt.Errorf("failed to delete transfer: %s", result.Message)
	}

	return nil
}

func (c *Client) ListFolder(ctx context.Context, folderID string) (*FolderListResponse, error) {
	endpoint := "/folder/list"
	if folderID != "" {
		endpoint = fmt.Sprintf("%s&id=%s", endpoint, folderID)
	}

	var result FolderListResponse
	if err := c.doJSONRequest(ctx, http.MethodGet, endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("listing folder: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("failed to list folder: %s", result.Message)
	}

	return &result, nil
}


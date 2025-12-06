// Package http provides HTTP client for server communication
package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jtsang4/nettune/internal/shared/types"
)

// Client is an HTTP client for the nettune server
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new HTTP client
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Response represents a server response
type Response struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *types.APIError `json:"error,omitempty"`
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(method, path string, body interface{}) (*Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response Response
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// ProbeEcho calls GET /probe/echo
func (c *Client) ProbeEcho() (*types.EchoResponse, error) {
	resp, err := c.doRequest("GET", "/probe/echo", nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result types.EchoResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ProbeDownload downloads data and returns the bytes received
func (c *Client) ProbeDownload(bytes int64) (int64, time.Duration, error) {
	start := time.Now()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/probe/download?bytes=%d", c.baseURL, bytes), nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	received, err := io.Copy(io.Discard, resp.Body)
	duration := time.Since(start)

	if err != nil {
		return received, duration, err
	}

	return received, duration, nil
}

// ProbeUpload uploads data and returns the result
func (c *Client) ProbeUpload(data []byte) (*types.UploadResponse, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/probe/upload", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, response.Error
	}

	var result types.UploadResponse
	if err := json.Unmarshal(response.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ProbeInfo calls GET /probe/info
func (c *Client) ProbeInfo() (*types.ServerInfo, error) {
	resp, err := c.doRequest("GET", "/probe/info", nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result types.ServerInfo
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListProfiles calls GET /profiles
func (c *Client) ListProfiles() ([]*types.ProfileMeta, error) {
	resp, err := c.doRequest("GET", "/profiles", nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result struct {
		Profiles []*types.ProfileMeta `json:"profiles"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return result.Profiles, nil
}

// GetProfile calls GET /profiles/:id
func (c *Client) GetProfile(id string) (*types.Profile, error) {
	resp, err := c.doRequest("GET", "/profiles/"+id, nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result types.Profile
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateProfile calls POST /profiles to create a new profile
func (c *Client) CreateProfile(profile *types.Profile) (*types.ProfileMeta, error) {
	resp, err := c.doRequest("POST", "/profiles", profile)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result types.ProfileMeta
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateSnapshot calls POST /sys/snapshot
func (c *Client) CreateSnapshot() (*types.Snapshot, error) {
	resp, err := c.doRequest("POST", "/sys/snapshot", nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result struct {
		SnapshotID   string             `json:"snapshot_id"`
		CurrentState *types.SystemState `json:"current_state"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}

	return &types.Snapshot{
		ID:    result.SnapshotID,
		State: result.CurrentState,
	}, nil
}

// Apply calls POST /sys/apply
func (c *Client) Apply(req *types.ApplyRequest) (*types.ApplyResult, error) {
	resp, err := c.doRequest("POST", "/sys/apply", req)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result types.ApplyResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Rollback calls POST /sys/rollback
func (c *Client) Rollback(req *types.RollbackRequest) (*types.RollbackResult, error) {
	resp, err := c.doRequest("POST", "/sys/rollback", req)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result types.RollbackResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetStatus calls GET /sys/status
func (c *Client) GetStatus() (*types.SystemStatus, error) {
	resp, err := c.doRequest("GET", "/sys/status", nil)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, resp.Error
	}

	var result types.SystemStatus
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

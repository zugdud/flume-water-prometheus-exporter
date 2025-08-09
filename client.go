package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// FlumeClient handles communication with the Flume API
type FlumeClient struct {
	baseURL      string
	httpClient   *http.Client
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	username     string
	password     string
	tokenExpiry  time.Time
}

// NewFlumeClient creates a new Flume API client
func NewFlumeClient(config *Config) *FlumeClient {
	return &FlumeClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
		username:     config.Username,
		password:     config.Password,
	}
}

// TokenResponse represents the response from the token endpoint
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// Device represents a Flume device
type Device struct {
	ID       string `json:"id"`
	Type     int    `json:"type"`
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
}

// QueryRequest represents a query request to the Flume API
type QueryRequest struct {
	Queries []Query `json:"queries"`
}

// Query represents a single query within a request
type Query struct {
	RequestID       string `json:"request_id"`
	Bucket          string `json:"bucket"`
	SinceDatetime   string `json:"since_datetime"`
	UntilDatetime   string `json:"until_datetime,omitempty"`
	GroupMultiplier int    `json:"group_multiplier,omitempty"`
}

// QueryResponse represents the response from a query
type QueryResponse struct {
	Count int `json:"count"`
	Data  []struct {
		QueryData [][]interface{} `json:"query_data"`
		RequestID string          `json:"request_id"`
		Bucket    string          `json:"bucket"`
	} `json:"data"`
}

// FlowRateResponse represents the current flow rate response
type FlowRateResponse struct {
	Value float64 `json:"value"`
	Units string  `json:"units"`
}

// DevicesResponse represents the response from the devices endpoint
type DevicesResponse struct {
	Count int      `json:"count"`
	Data  []Device `json:"data"`
}

// isTokenExpired checks if the current access token is expired or will expire soon
func (c *FlumeClient) isTokenExpired() bool {
	// Consider token expired if it expires within the next 5 minutes
	return time.Now().Add(5 * time.Minute).After(c.tokenExpiry)
}

// ensureValidToken ensures we have a valid access token, refreshing if necessary
func (c *FlumeClient) ensureValidToken() error {
	if c.accessToken == "" || c.isTokenExpired() {
		if c.refreshToken != "" {
			// Try to refresh the token first
			if err := c.refreshAccessToken(); err != nil {
				// If refresh fails, fall back to full authentication
				log.Printf("Token refresh failed, falling back to full authentication: %v", err)
				return c.Authenticate()
			}
		} else {
			// No refresh token, need full authentication
			return c.Authenticate()
		}
	}
	return nil
}

// refreshAccessToken refreshes the access token using the refresh token
func (c *FlumeClient) refreshAccessToken() error {
	tokenData := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"refresh_token": c.refreshToken,
	}

	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh token request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/oauth/token", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create refresh token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send refresh token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode refresh token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		c.refreshToken = tokenResp.RefreshToken
	}
	// Set new expiry time
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// Authenticate obtains access token from the Flume API
func (c *FlumeClient) Authenticate() error {
	tokenData := map[string]string{
		"grant_type":    "password",
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"username":      c.username,
		"password":      c.password,
	}

	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/oauth/token", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	c.accessToken = tokenResp.AccessToken
	c.refreshToken = tokenResp.RefreshToken
	// Set expiry time
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// GetDevices retrieves all devices for the authenticated user
func (c *FlumeClient) GetDevices() ([]Device, error) {
	// Ensure we have a valid token before making the request
	if err := c.ensureValidToken(); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	req, err := http.NewRequest("GET", c.baseURL+"/me/devices", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create devices request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send devices request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("devices request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var devicesResp DevicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&devicesResp); err != nil {
		return nil, fmt.Errorf("failed to decode devices response: %w", err)
	}

	return devicesResp.Data, nil
}

// GetCurrentFlowRate retrieves the current flow rate for a device
func (c *FlumeClient) GetCurrentFlowRate(deviceID string) (*FlowRateResponse, error) {
	// Ensure we have a valid token before making the request
	if err := c.ensureValidToken(); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	url := fmt.Sprintf("%s/me/devices/%s/current_interval", c.baseURL, deviceID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create flow rate request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send flow rate request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("flow rate request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var flowRate FlowRateResponse
	if err := json.NewDecoder(resp.Body).Decode(&flowRate); err != nil {
		return nil, fmt.Errorf("failed to decode flow rate response: %w", err)
	}

	return &flowRate, nil
}

// QueryWaterUsage queries water usage data for a device
func (c *FlumeClient) QueryWaterUsage(deviceID string, bucket string, since time.Time, until *time.Time) (*QueryResponse, error) {
	// Ensure we have a valid token before making the request
	if err := c.ensureValidToken(); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	query := Query{
		RequestID:     "water_usage",
		Bucket:        bucket,
		SinceDatetime: since.Format("2006-01-02 15:04:05"),
	}

	if until != nil {
		query.UntilDatetime = until.Format("2006-01-02 15:04:05")
	}

	queryReq := QueryRequest{
		Queries: []Query{query},
	}

	jsonData, err := json.Marshal(queryReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query request: %w", err)
	}

	url := fmt.Sprintf("%s/me/devices/%s/query", c.baseURL, deviceID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create query request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send query request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode query response: %w", err)
	}

	return &queryResp, nil
}

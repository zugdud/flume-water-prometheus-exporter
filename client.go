package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	tokenFile    string
	rateLimiter  *RateLimiter
}

// TokenData represents the token data structure for persistence
type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiryTime   time.Time `json:"expiry_time"`
	Username     string    `json:"username"`
	ClientID     string    `json:"client_id"`
}

// NewFlumeClient creates a new Flume API client
func NewFlumeClient(config *Config) *FlumeClient {
	// Create token file path in user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: Could not determine home directory, using current directory: %v", err)
		homeDir = "."
	}
	tokenFile := filepath.Join(homeDir, ".flume_exporter_tokens.json")

	client := &FlumeClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
		username:     config.Username,
		password:     config.Password,
		tokenFile:    tokenFile,
		rateLimiter:  NewRateLimiter(config.APIMinInterval),
	}

	// Try to load existing tokens
	client.loadTokens()

	return client
}

// loadTokens attempts to load tokens from the token file
func (c *FlumeClient) loadTokens() {
	if c.tokenFile == "" {
		return
	}

	data, err := os.ReadFile(c.tokenFile)
	if err != nil {
		log.Printf("No existing tokens found (this is normal for first run): %v", err)
		return
	}

	var tokenData TokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		log.Printf("Failed to parse token file: %v", err)
		return
	}

	// Validate that tokens belong to the current user/client
	if tokenData.Username != c.username || tokenData.ClientID != c.clientID {
		log.Printf("Token file contains tokens for different user/client, ignoring")
		return
	}

	// Check if tokens are still valid
	if time.Now().Before(tokenData.ExpiryTime) {
		c.accessToken = tokenData.AccessToken
		c.refreshToken = tokenData.RefreshToken
		c.tokenExpiry = tokenData.ExpiryTime
		log.Printf("Loaded valid tokens from file, expires at: %v", c.tokenExpiry)
	} else {
		log.Printf("Tokens in file are expired, will need to re-authenticate")
	}
}

// saveTokens saves the current tokens to the token file
func (c *FlumeClient) saveTokens() error {
	if c.tokenFile == "" {
		return nil
	}

	tokenData := TokenData{
		AccessToken:  c.accessToken,
		RefreshToken: c.refreshToken,
		ExpiryTime:   c.tokenExpiry,
		Username:     c.username,
		ClientID:     c.clientID,
	}

	data, err := json.MarshalIndent(tokenData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(c.tokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// Write with restrictive permissions
	if err := os.WriteFile(c.tokenFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	log.Printf("Tokens saved to: %s", c.tokenFile)
	return nil
}

// TokenResponse represents the response from the Flume OAuth token endpoint
type TokenResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		TokenType    string `json:"token_type"`
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	} `json:"data"`
	Count int `json:"count"`
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

// DailyTotalWaterUsageResponse represents the response from a daily total water usage query
type DailyTotalWaterUsageResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		RequestID string `json:"request_id"`
		Data      map[string][]struct {
			DateTime string  `json:"datetime"`
			Value    float64 `json:"value"`
		} `json:"-"`
	} `json:"data"`
	Count int `json:"count"`
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
	log.Printf("ensureValidToken: accessToken='%s', refreshToken='%s', tokenExpiry=%v",
		c.accessToken, c.refreshToken, c.tokenExpiry)

	if c.accessToken == "" || c.isTokenExpired() {
		log.Printf("ensureValidToken: Token is empty or expired, need to authenticate")
		if c.refreshToken != "" {
			// Try to refresh the token first
			log.Printf("ensureValidToken: Attempting token refresh...")
			if err := c.refreshAccessToken(); err != nil {
				// If refresh fails, fall back to full authentication with retry
				log.Printf("Token refresh failed, falling back to full authentication: %v", err)
				return c.AuthenticateWithRetry(3)
			}
		} else {
			// No refresh token, need full authentication
			log.Printf("ensureValidToken: No refresh token, performing full authentication...")
			return c.AuthenticateWithRetry(3)
		}
	} else {
		log.Printf("ensureValidToken: Token is valid, expiry: %v", c.tokenExpiry)
	}
	return nil
}

// refreshAccessToken refreshes the access token using the refresh token
func (c *FlumeClient) refreshAccessToken() error {
	log.Printf("refreshAccessToken: Attempting to refresh token...")

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

	log.Printf("refreshAccessToken: Sending refresh request to %s", c.baseURL+"/oauth/token")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send refresh token request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("refreshAccessToken: Response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("refreshAccessToken: Error response body: %s", string(body))
		return fmt.Errorf("refresh token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode refresh token response: %w", err)
	}

	// Validate response structure
	if !tokenResp.Success || len(tokenResp.Data) == 0 {
		return fmt.Errorf("refresh response indicates failure or no data: success=%v, count=%d", tokenResp.Success, tokenResp.Count)
	}

	refreshTokenData := tokenResp.Data[0] // Get first token from data array

	log.Printf("refreshAccessToken: Successfully refreshed token, expires in %d seconds", refreshTokenData.ExpiresIn)

	c.accessToken = refreshTokenData.AccessToken
	if refreshTokenData.RefreshToken != "" {
		c.refreshToken = refreshTokenData.RefreshToken
	}
	// Set new expiry time
	c.tokenExpiry = time.Now().Add(time.Duration(refreshTokenData.ExpiresIn) * time.Second)

	// Save the refreshed tokens
	if err := c.saveTokens(); err != nil {
		log.Printf("Warning: Failed to save refreshed tokens: %v", err)
	}

	return nil
}

// Authenticate obtains access token from the Flume API
func (c *FlumeClient) Authenticate() error {
	log.Printf("Authenticate: Starting authentication with username: %s", c.username)

	tokenData := map[string]string{
		"grant_type":    "password",
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"username":      c.username,
		"password":      c.password,
	}

	log.Printf("Authenticate: Token request data: %+v", map[string]string{
		"grant_type": "password",
		"client_id":  c.clientID,
		"username":   c.username,
		"password":   "***",
	})

	jsonData, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/oauth/token", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	log.Printf("Authenticate: Sending request to %s", c.baseURL+"/oauth/token")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("Authenticate: Response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Authenticate: Error response body: %s", string(body))
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Log the response body for debugging
	body, _ := io.ReadAll(resp.Body)
	log.Printf("Authenticate: Response body: %s", string(body))
	log.Printf("Authenticate: Response headers: %+v", resp.Header)

	// Try to parse as generic JSON first to see the structure
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(body, &rawResponse); err != nil {
		log.Printf("Authenticate: Failed to parse as generic JSON: %v", err)
	} else {
		log.Printf("Authenticate: Raw response structure: %+v", rawResponse)
	}

	// Create a new reader since we consumed the body
	bodyReader := bytes.NewReader(body)

	var tokenResp TokenResponse
	if err := json.NewDecoder(bodyReader).Decode(&tokenResp); err != nil {
		log.Printf("Authenticate: Failed to decode response: %v", err)
		log.Printf("Authenticate: Raw response: %s", string(body))
		return fmt.Errorf("failed to decode token response: %w", err)
	}

	// Validate response structure
	if !tokenResp.Success || len(tokenResp.Data) == 0 {
		return fmt.Errorf("authentication response indicates failure or no data: success=%v, count=%d", tokenResp.Success, tokenResp.Count)
	}

	authTokenData := tokenResp.Data[0] // Get first token from data array

	log.Printf("Authenticate: Successfully obtained token, expires in %d seconds", authTokenData.ExpiresIn)
	log.Printf("Authenticate: Token type: %s", authTokenData.TokenType)
	log.Printf("Authenticate: Access token length: %d", len(authTokenData.AccessToken))
	log.Printf("Authenticate: Refresh token length: %d", len(authTokenData.RefreshToken))

	c.accessToken = authTokenData.AccessToken
	c.refreshToken = authTokenData.RefreshToken
	// Set expiry time
	c.tokenExpiry = time.Now().Add(time.Duration(authTokenData.ExpiresIn) * time.Second)

	// Validate that we actually got tokens
	if c.accessToken == "" {
		return fmt.Errorf("authentication succeeded but returned empty access token")
	}
	if c.refreshToken == "" {
		log.Printf("Warning: No refresh token received")
	}

	// Save the tokens for future use
	if err := c.saveTokens(); err != nil {
		log.Printf("Warning: Failed to save tokens: %v", err)
	}

	return nil
}

// clearTokens clears the current tokens and removes the token file
func (c *FlumeClient) clearTokens() {
	c.accessToken = ""
	c.refreshToken = ""
	c.tokenExpiry = time.Time{}

	if c.tokenFile != "" {
		if err := os.Remove(c.tokenFile); err != nil {
			log.Printf("Warning: Failed to remove token file: %v", err)
		} else {
			log.Printf("Cleared invalid tokens and removed token file")
		}
	}
}

// AuthenticateWithRetry attempts authentication with retry logic
func (c *FlumeClient) AuthenticateWithRetry(maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Authentication attempt %d/%d", attempt, maxRetries)

		if err := c.Authenticate(); err != nil {
			lastErr = err
			log.Printf("Authentication attempt %d failed: %v", attempt, maxRetries)

			if attempt < maxRetries {
				// Clear any partial tokens and wait before retry
				c.clearTokens()
				waitTime := time.Duration(attempt) * 5 * time.Second
				log.Printf("Waiting %v before retry...", waitTime)
				time.Sleep(waitTime)
			}
		} else {
			log.Printf("Authentication successful on attempt %d", attempt)
			return nil
		}
	}

	return fmt.Errorf("authentication failed after %d attempts, last error: %w", maxRetries, lastErr)
}

// GetDevices retrieves all devices for the authenticated user
func (c *FlumeClient) GetDevices() ([]Device, error) {
	// Apply rate limiting
	c.rateLimiter.Wait()

	// Ensure we have a valid token before making the request
	if err := c.ensureValidToken(); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	log.Printf("GetDevices: Using access token: %s...", c.accessToken[:10])

	req, err := http.NewRequest("GET", c.baseURL+"/me/devices", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create devices request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if len(c.accessToken) >= 10 {
		log.Printf("GetDevices: Set Authorization header: Bearer %s...", c.accessToken[:10])
	} else {
		log.Printf("GetDevices: Set Authorization header: Bearer %s", c.accessToken)
	}
	log.Printf("GetDevices: Full Authorization header: %s", req.Header.Get("Authorization"))

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
// Using the direct flow rate endpoint: /users/{user_id}/devices/{device_id}/query/active
func (c *FlumeClient) GetCurrentFlowRate(deviceID string) (*FlowRateResponse, error) {
	// Apply rate limiting
	c.rateLimiter.Wait()

	// Ensure we have a valid token before making the request
	if err := c.ensureValidToken(); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	// Use the direct flow rate endpoint
	// First get the user ID from the /me endpoint
	meURL := fmt.Sprintf("%s/me", c.baseURL)
	meReq, err := http.NewRequest("GET", meURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create me request: %w", err)
	}

	meReq.Header.Set("Accept", "application/json")
	meReq.Header.Set("Authorization", "Bearer "+c.accessToken)

	meResp, err := c.httpClient.Do(meReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send me request: %w", err)
	}
	defer meResp.Body.Close()

	if meResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(meResp.Body)
		return nil, fmt.Errorf("me request failed with status %d: %s", meResp.StatusCode, string(body))
	}

	// Parse user ID from response
	meBody, _ := io.ReadAll(meResp.Body)
	log.Printf("GetCurrentFlowRate: /me response body: %s", string(meBody))

	// Try to parse as generic JSON first to see the structure
	var meData map[string]interface{}
	if err := json.Unmarshal(meBody, &meData); err != nil {
		return nil, fmt.Errorf("failed to decode me response: %w", err)
	}

	log.Printf("GetCurrentFlowRate: /me response structure: %+v", meData)

	// Extract user ID from the response
	var userID int
	if data, ok := meData["data"].([]interface{}); ok && len(data) > 0 {
		if firstItem, ok := data[0].(map[string]interface{}); ok {
			// Try to get user ID from the 'id' field first (as shown in the /me response)
			if userIDFloat, ok := firstItem["id"].(float64); ok {
				userID = int(userIDFloat)
				log.Printf("GetCurrentFlowRate: Found user ID in 'id' field: %d", userID)
			} else if userIDInt, ok := firstItem["id"].(int); ok {
				userID = userIDInt
				log.Printf("GetCurrentFlowRate: Found user ID in 'id' field: %d", userID)
			} else if userIDStr, ok := firstItem["id"].(string); ok {
				// Try to parse string user ID
				if parsed, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil || parsed != 1 {
					return nil, fmt.Errorf("failed to parse id string '%s': %w", userIDStr, err)
				}
				log.Printf("GetCurrentFlowRate: Found user ID in 'id' field (string): %d", userID)
			} else {
				// Fallback: try to get from 'user_id' field
				if userIDFloat, ok := firstItem["user_id"].(float64); ok {
					userID = int(userIDFloat)
					log.Printf("GetCurrentFlowRate: Found user ID in 'user_id' field: %d", userID)
				} else if userIDInt, ok := firstItem["user_id"].(int); ok {
					userID = userIDInt
					log.Printf("GetCurrentFlowRate: Found user ID in 'user_id' field: %d", userID)
				} else if userIDStr, ok := firstItem["user_id"].(string); ok {
					// Try to parse string user ID
					if parsed, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil || parsed != 1 {
						return nil, fmt.Errorf("failed to parse user_id string '%s': %w", userIDStr, err)
					}
					log.Printf("GetCurrentFlowRate: Found user ID in 'user_id' field (string): %d", userID)
				} else {
					log.Printf("GetCurrentFlowRate: Neither 'id' nor 'user_id' field found in /me response")
					// Final fallback: try to extract from JWT token
					if userIDFromToken := c.extractUserIDFromToken(); userIDFromToken > 0 {
						userID = userIDFromToken
						log.Printf("GetCurrentFlowRate: Using user ID from JWT token: %d", userID)
					} else {
						return nil, fmt.Errorf("could not extract user ID from /me response or JWT token")
					}
				}
			}
		}
	}

	if userID == 0 {
		return nil, fmt.Errorf("invalid user ID (0) extracted from /me response")
	}

	log.Printf("GetCurrentFlowRate: Extracted user ID: %d", userID)
	url := fmt.Sprintf("%s/users/%d/devices/%s/query/active", c.baseURL, userID, deviceID)
	log.Printf("GetCurrentFlowRate: Querying URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create flow rate request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
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

	// Read and log the response body for debugging
	body, _ := io.ReadAll(resp.Body)
	log.Printf("GetCurrentFlowRate: Response status: %d", resp.StatusCode)
	log.Printf("GetCurrentFlowRate: Response body: %s", string(body))

	// Parse the response using the correct structure
	var flowRateResp struct {
		Success bool   `json:"success"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    []struct {
			Active   bool    `json:"active"`
			GPM      float64 `json:"gpm"`
			DateTime string  `json:"datetime"`
		} `json:"data"`
		Count int `json:"count"`
	}

	if err := json.Unmarshal(body, &flowRateResp); err != nil {
		return nil, fmt.Errorf("failed to decode flow rate response: %w", err)
	}

	if !flowRateResp.Success {
		return nil, fmt.Errorf("flow rate response indicates failure: %s", flowRateResp.Message)
	}

	if len(flowRateResp.Data) == 0 {
		log.Printf("GetCurrentFlowRate: No flow rate data returned")
		return &FlowRateResponse{
			Value: 0.0,
			Units: "gallons_per_minute",
		}, nil
	}

	// Get the most recent flow rate data
	flowRateData := flowRateResp.Data[0]
	log.Printf("GetCurrentFlowRate: Flow rate data - Active: %v, GPM: %f, DateTime: %s",
		flowRateData.Active, flowRateData.GPM, flowRateData.DateTime)

	// Return the flow rate in gallons per minute
	return &FlowRateResponse{
		Value: flowRateData.GPM,
		Units: "gallons_per_minute",
	}, nil
}

// QueryDailyTotalWaterUsage queries daily total water usage data for a device over a date range
func (c *FlumeClient) QueryDailyTotalWaterUsage(deviceID string, since time.Time, until time.Time) (*DailyTotalWaterUsageResponse, error) {
	// Apply rate limiting
	c.rateLimiter.Wait()

	// Ensure we have a valid token before making the request
	if err := c.ensureValidToken(); err != nil {
		return nil, fmt.Errorf("failed to ensure valid token: %w", err)
	}

	query := Query{
		RequestID:     "daily_total_water_usage",
		Bucket:        "DAY",
		SinceDatetime: since.Format("2006-01-02 15:04:05"),
		UntilDatetime: until.Format("2006-01-02 15:04:05"),
	}

	queryReq := QueryRequest{
		Queries: []Query{query},
	}

	jsonData, err := json.Marshal(queryReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query request: %w", err)
	}

	url := fmt.Sprintf("%s/me/devices/%s/query", c.baseURL, deviceID)
	log.Printf("QueryDailyTotalWaterUsage: Querying URL: %s", url)
	log.Printf("QueryDailyTotalWaterUsage: Request body: %s", string(jsonData))
	log.Printf("QueryDailyTotalWaterUsage: Since: %v, Until: %v", since, until)

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

	// Read and log the response body for debugging
	body, _ := io.ReadAll(resp.Body)
	log.Printf("QueryDailyTotalWaterUsage: Response status: %d", resp.StatusCode)
	log.Printf("QueryDailyTotalWaterUsage: Response body: %s", string(body))

	// Create a new reader since we consumed the body
	bodyReader := bytes.NewReader(body)

	var dailyTotalResp DailyTotalWaterUsageResponse
	if err := json.NewDecoder(bodyReader).Decode(&dailyTotalResp); err != nil {
		return nil, fmt.Errorf("failed to decode query response: %w", err)
	}

	log.Printf("QueryDailyTotalWaterUsage: Parsed response - Count: %d, Data entries: %d",
		dailyTotalResp.Count, len(dailyTotalResp.Data))

	return &dailyTotalResp, nil
}

// QueryWaterUsage queries water usage data for a device
func (c *FlumeClient) QueryWaterUsage(deviceID string, bucket string, since time.Time, until *time.Time) (*QueryResponse, error) {
	// Apply rate limiting
	c.rateLimiter.Wait()

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
	log.Printf("QueryWaterUsage: Querying URL: %s", url)
	log.Printf("QueryWaterUsage: Request body: %s", string(jsonData))
	log.Printf("QueryWaterUsage: Bucket: %s, Since: %v, Until: %v", bucket, since, until)

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

	// Read and log the response body for debugging
	body, _ := io.ReadAll(resp.Body)
	log.Printf("QueryWaterUsage: Response status: %d", resp.StatusCode)
	log.Printf("QueryWaterUsage: Response body: %s", string(body))

	// Create a new reader since we consumed the body
	bodyReader := bytes.NewReader(body)

	var queryResp QueryResponse
	if err := json.NewDecoder(bodyReader).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode query response: %w", err)
	}

	log.Printf("QueryWaterUsage: Parsed response - Count: %d, Data entries: %d",
		queryResp.Count, len(queryResp.Data))

	if len(queryResp.Data) > 0 && len(queryResp.Data[0].QueryData) > 0 {
		log.Printf("QueryWaterUsage: First data point: %+v", queryResp.Data[0].QueryData[0])
	}

	return &queryResp, nil
}

// ValidateAuthentication checks if the current authentication is working by making a test API call
func (c *FlumeClient) ValidateAuthentication() error {
	if c.accessToken == "" {
		return fmt.Errorf("no access token available")
	}

	// Make a simple API call to test authentication
	req, err := http.NewRequest("GET", c.baseURL+"/me", nil)
	if err != nil {
		return fmt.Errorf("failed to create validation request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send validation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// Token is invalid, clear it and force re-authentication
		log.Printf("Validation failed: Token is unauthorized, clearing tokens")
		c.clearTokens()
		return fmt.Errorf("authentication token is invalid")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("validation request failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Authentication validation successful")
	return nil
}

// GetAuthenticationStatus returns the current authentication status
func (c *FlumeClient) GetAuthenticationStatus() map[string]interface{} {
	status := map[string]interface{}{
		"has_access_token":  c.accessToken != "",
		"has_refresh_token": c.refreshToken != "",
		"token_expiry":      c.tokenExpiry,
		"is_expired":        c.isTokenExpired(),
		"token_file":        c.tokenFile,
	}

	if c.accessToken != "" {
		status["access_token_length"] = len(c.accessToken)
		status["access_token_preview"] = c.accessToken[:min(10, len(c.accessToken))] + "..."
	}

	if c.refreshToken != "" {
		status["refresh_token_length"] = len(c.refreshToken)
		status["refresh_token_preview"] = c.refreshToken[:min(10, len(c.refreshToken))] + "..."
	}

	return status
}

// extractUserIDFromToken extracts the user ID from the JWT access token
func (c *FlumeClient) extractUserIDFromToken() int {
	if c.accessToken == "" {
		return 0
	}

	// JWT tokens have 3 parts separated by dots
	parts := strings.Split(c.accessToken, ".")
	if len(parts) != 3 {
		return 0
	}

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0
	}

	// Parse the JSON payload
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0
	}

	// Extract user_id from claims
	if userID, ok := claims["user_id"]; ok {
		switch v := userID.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			if parsed, err := strconv.Atoi(v); err == nil {
				return parsed
			}
		}
	}

	return 0
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

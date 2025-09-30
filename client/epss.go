package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"epss-fetcher-app/config"
	"epss-fetcher-app/models"
)

type EPSSClient struct {
	client      *http.Client
	config      *config.APIConfig
	rateLimiter chan struct{}
}

func NewEPSSClient(cfg *config.APIConfig) *EPSSClient {
	// Create rate limiter channel
	rateLimiter := make(chan struct{}, 1)

	// Fill the rate limiter
	rateLimiter <- struct{}{}

	// Start rate limiter goroutine
	go func() {
		for {
			time.Sleep(cfg.RateLimit)
			select {
			case rateLimiter <- struct{}{}:
			default:
				// Channel is full, skip
			}
		}
	}()

	return &EPSSClient{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		config:      cfg,
		rateLimiter: rateLimiter,
	}
}

func (c *EPSSClient) FetchEPSSData(ctx context.Context, offset, limit int) (*models.EPSSResponse, error) {
	// Wait for rate limiter
	select {
	case <-c.rateLimiter:
		// Rate limit satisfied
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	url := fmt.Sprintf("%s?offset=%d&limit=%d", c.config.BaseURL, offset, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "epss-fetcher/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response models.EPSSResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Add timestamp to each record
	now := time.Now()
	for i := range response.Data {
		response.Data[i].Timestamp = now
	}

	return &response, nil
}

func (c *EPSSClient) FetchEPSSDataByDate(ctx context.Context, date string, offset, limit int) (*models.EPSSResponse, error) {
	// Wait for rate limiter
	select {
	case <-c.rateLimiter:
		// Rate limit satisfied
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var url string
	if date != "" {
		url = fmt.Sprintf("%s?date=%s&offset=%d&limit=%d", c.config.BaseURL, date, offset, limit)
	} else {
		url = fmt.Sprintf("%s?offset=%d&limit=%d", c.config.BaseURL, offset, limit)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "epss-fetcher/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response models.EPSSResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Add timestamp to each record
	now := time.Now()
	for i := range response.Data {
		response.Data[i].Timestamp = now
	}

	return &response, nil
}

func (c *EPSSClient) GetTotalRecords(ctx context.Context) (int, error) {
	// Make a single request to get total count
	resp, err := c.FetchEPSSData(ctx, 0, 1)
	if err != nil {
		return 0, fmt.Errorf("failed to get total records: %w", err)
	}
	return resp.Total, nil
}

func (c *EPSSClient) GetTotalRecordsForDate(ctx context.Context, date string) (int, error) {
	// Make a single request to get total count for specific date
	resp, err := c.FetchEPSSDataByDate(ctx, date, 0, 1)
	if err != nil {
		return 0, fmt.Errorf("failed to get total records for date %s: %w", date, err)
	}
	return resp.Total, nil
}

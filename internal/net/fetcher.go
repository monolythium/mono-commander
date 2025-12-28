// Package net provides network operations for mono-commander.
package net

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPFetcher implements the Fetcher interface using HTTP.
type HTTPFetcher struct {
	Client  *http.Client
	Timeout time.Duration
}

// NewHTTPFetcher creates a new HTTPFetcher with sensible defaults.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		Timeout: 30 * time.Second,
	}
}

// Fetch downloads data from a URL.
func (f *HTTPFetcher) Fetch(url string) ([]byte, error) {
	resp, err := f.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s: HTTP %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", url, err)
	}

	return data, nil
}

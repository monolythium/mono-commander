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

// ipDetectionServices is a list of services that return the public IP as plain text.
var ipDetectionServices = []string{
	"https://ifconfig.me/ip",
	"https://api.ipify.org",
	"https://ipinfo.io/ip",
	"https://icanhazip.com",
}

// DetectPublicIP attempts to detect the public IP address using external services.
// It tries multiple services and returns the first successful result.
// Returns empty string if detection fails.
func DetectPublicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}

	for _, url := range ipDetectionServices {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		ip := string(data)
		// Clean up the response (remove whitespace/newlines)
		ip = trimWhitespace(ip)

		// Validate: should look like an IP address AND be a public IP
		if isValidIP(ip) && IsPublicIP(ip) {
			return ip
		}
	}

	return ""
}

// trimWhitespace removes leading/trailing whitespace and newlines.
func trimWhitespace(s string) string {
	result := s
	for len(result) > 0 && (result[0] == ' ' || result[0] == '\t' || result[0] == '\n' || result[0] == '\r') {
		result = result[1:]
	}
	for len(result) > 0 && (result[len(result)-1] == ' ' || result[len(result)-1] == '\t' || result[len(result)-1] == '\n' || result[len(result)-1] == '\r') {
		result = result[:len(result)-1]
	}
	return result
}

// isValidIP checks if a string looks like a valid IPv4 or IPv6 address.
func isValidIP(s string) bool {
	if len(s) < 7 || len(s) > 45 {
		return false
	}
	// Simple check: must contain dots (IPv4) or colons (IPv6)
	hasDots := false
	hasColons := false
	for _, c := range s {
		if c == '.' {
			hasDots = true
		}
		if c == ':' {
			hasColons = true
		}
		// Must only contain valid IP characters
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '.' || c == ':') {
			return false
		}
	}
	return hasDots || hasColons
}

// IsPublicIP checks if an IP address is a public (routable) IP.
// Returns false for localhost, private ranges, and link-local addresses.
func IsPublicIP(ip string) bool {
	if ip == "" {
		return false
	}

	// Reject localhost
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return false
	}

	// Reject common private IPv4 ranges
	privateRanges := []string{
		"10.",     // 10.0.0.0/8
		"172.16.", // 172.16.0.0/12
		"172.17.",
		"172.18.",
		"172.19.",
		"172.20.",
		"172.21.",
		"172.22.",
		"172.23.",
		"172.24.",
		"172.25.",
		"172.26.",
		"172.27.",
		"172.28.",
		"172.29.",
		"172.30.",
		"172.31.",
		"192.168.", // 192.168.0.0/16
		"169.254.", // Link-local
		"127.",     // Loopback
	}

	for _, prefix := range privateRanges {
		if len(ip) >= len(prefix) && ip[:len(prefix)] == prefix {
			return false
		}
	}

	return true
}

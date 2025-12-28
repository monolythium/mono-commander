// Package mesh provides Mesh/Rosetta API sidecar management for mono-commander.
package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/monolythium/mono-commander/internal/core"
)

// HealthStatus represents the health status of the Mesh/Rosetta API.
type HealthStatus struct {
	// Healthy indicates if the service is responding.
	Healthy bool `json:"healthy"`

	// Method indicates how health was determined.
	Method string `json:"method"`

	// ListenAddress is the address the service is listening on.
	ListenAddress string `json:"listen_address"`

	// ResponseTime is the response time in milliseconds.
	ResponseTime int64 `json:"response_time_ms"`

	// Details contains additional information from the health check.
	Details map[string]interface{} `json:"details,omitempty"`

	// Error contains any error message.
	Error string `json:"error,omitempty"`
}

// HealthChecker performs health checks on the Mesh/Rosetta API.
type HealthChecker struct {
	// Client is the HTTP client to use for requests.
	Client *http.Client

	// Timeout is the timeout for health checks.
	Timeout time.Duration
}

// NewHealthChecker creates a new HealthChecker with default settings.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
		Timeout: 5 * time.Second,
	}
}

// Check performs a health check on the Mesh/Rosetta API at the given address.
// It tries multiple methods in order:
// 1. GET /health (preferred if supported)
// 2. POST /network/list (Rosetta standard endpoint)
// 3. TCP port check (fallback)
func (hc *HealthChecker) Check(ctx context.Context, listenAddress string) *HealthStatus {
	status := &HealthStatus{
		ListenAddress: listenAddress,
	}

	start := time.Now()

	// Normalize address to URL format
	url := normalizeAddressToURL(listenAddress)

	// Try /health endpoint first
	if err := hc.checkHealthEndpoint(ctx, url, status); err == nil {
		status.Healthy = true
		status.Method = "health_endpoint"
		status.ResponseTime = time.Since(start).Milliseconds()
		return status
	}

	// Try Rosetta /network/list endpoint
	if err := hc.checkNetworkList(ctx, url, status); err == nil {
		status.Healthy = true
		status.Method = "network_list"
		status.ResponseTime = time.Since(start).Milliseconds()
		return status
	}

	// Fall back to TCP port check
	if err := hc.checkTCPPort(ctx, listenAddress); err == nil {
		status.Healthy = true
		status.Method = "tcp_port"
		status.ResponseTime = time.Since(start).Milliseconds()
		status.Details = map[string]interface{}{
			"note": "TCP port is open, but HTTP health check failed. Service may be starting up.",
		}
		return status
	}

	status.Healthy = false
	status.Method = "tcp_port"
	status.Error = "service not responding"
	status.ResponseTime = time.Since(start).Milliseconds()
	return status
}

// checkHealthEndpoint tries to GET /health.
func (hc *HealthChecker) checkHealthEndpoint(ctx context.Context, baseURL string, status *HealthStatus) error {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := hc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	// Try to parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		status.Details = result
	}

	return nil
}

// checkNetworkList tries to POST to /network/list (Rosetta standard).
func (hc *HealthChecker) checkNetworkList(ctx context.Context, baseURL string, status *HealthStatus) error {
	// Rosetta /network/list expects an empty JSON object
	body := strings.NewReader("{}")
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/network/list", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := hc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("network/list returned status %d", resp.StatusCode)
	}

	// Try to parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		status.Details = result
	}

	return nil
}

// checkTCPPort tries to connect to the TCP port.
func (hc *HealthChecker) checkTCPPort(ctx context.Context, address string) error {
	// Extract host:port from address
	addr := address
	if strings.HasPrefix(addr, "http://") {
		addr = strings.TrimPrefix(addr, "http://")
	}
	if strings.HasPrefix(addr, "https://") {
		addr = strings.TrimPrefix(addr, "https://")
	}

	// Default port if not specified
	if !strings.Contains(addr, ":") {
		addr += ":8080"
	}

	d := net.Dialer{Timeout: hc.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// normalizeAddressToURL converts an address to a URL format.
func normalizeAddressToURL(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return strings.TrimSuffix(address, "/")
	}

	// Assume http for local addresses
	if strings.HasPrefix(address, "0.0.0.0:") || strings.HasPrefix(address, "127.0.0.1:") || strings.HasPrefix(address, "localhost:") {
		address = strings.Replace(address, "0.0.0.0", "127.0.0.1", 1)
		return "http://" + address
	}

	// Check if it's just a port
	if !strings.Contains(address, ":") {
		return "http://127.0.0.1:" + address
	}

	return "http://" + address
}

// CheckWithConfig performs a health check using the configuration.
func (hc *HealthChecker) CheckWithConfig(ctx context.Context, cfg *Config) *HealthStatus {
	return hc.Check(ctx, cfg.ListenAddress)
}

// CheckResult contains the combined health check result.
type CheckResult struct {
	// ServiceHealth is the health of the Mesh/Rosetta API itself.
	ServiceHealth *HealthStatus `json:"service_health"`

	// SystemdStatus is the systemd service status (if available).
	SystemdStatus *ServiceStatus `json:"systemd_status,omitempty"`

	// ConfigExists indicates if the config file exists.
	ConfigExists bool `json:"config_exists"`

	// BinaryExists indicates if the binary is installed.
	BinaryExists bool `json:"binary_exists"`
}

// FullCheck performs a comprehensive health check.
func FullCheck(ctx context.Context, network string, home string, netName core.NetworkName) *CheckResult {
	result := &CheckResult{}

	// Check if config exists
	result.ConfigExists = ConfigExists(home, netName)

	// Check if binary exists
	result.BinaryExists = BinaryExists(false) || BinaryExists(true)

	// Get systemd status
	if IsSystemdAvailable() {
		result.SystemdStatus = GetServiceStatus(network)
	}

	// Check service health
	if result.ConfigExists {
		cfg, err := LoadConfig(home, netName)
		if err == nil {
			hc := NewHealthChecker()
			result.ServiceHealth = hc.CheckWithConfig(ctx, cfg)
		}
	}

	return result
}

// BinaryExists checks if the mesh binary exists at the expected path.
func BinaryExists(useSystemPath bool) bool {
	path := BinaryInstallPath(useSystemPath)
	_, err := os.Stat(path)
	return err == nil
}

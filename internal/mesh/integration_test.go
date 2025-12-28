package mesh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/monolythium/mono-commander/internal/core"
)

// TestInstallDryRunFlow tests the complete install dry-run workflow.
func TestInstallDryRunFlow(t *testing.T) {
	// Create test server that serves a mock binary
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("mock binary content"))
	}))
	defer server.Close()

	opts := InstallOptions{
		URL:           server.URL + "/mono-mesh-rosetta",
		SHA256:        "abc123def456",
		Version:       "v0.1.0",
		UseSystemPath: false,
		DryRun:        true,
	}

	result := Install(opts)

	// Verify dry-run success
	if !result.Success {
		t.Errorf("Install dry-run failed: %v", result.Error)
	}

	// Verify no actual download
	if result.Downloaded {
		t.Error("Install dry-run should not download")
	}

	// Verify steps are populated
	if len(result.Steps) == 0 {
		t.Error("Install dry-run should have steps")
	}

	// Verify version is set
	if result.Version != "v0.1.0" {
		t.Errorf("Install dry-run Version = %s, want v0.1.0", result.Version)
	}

	// Verify install path is set
	if result.InstallPath == "" {
		t.Error("Install dry-run should set InstallPath")
	}
}

// TestEnableDisableDryRunFlow tests the enable/disable dry-run workflow.
func TestEnableDisableDryRunFlow(t *testing.T) {
	networks := []string{"Localnet", "Sprintnet", "Testnet", "Mainnet"}

	for _, network := range networks {
		t.Run(network, func(t *testing.T) {
			// Test enable dry-run
			enableResult, err := EnableService(network, true)
			if err != nil {
				t.Errorf("EnableService(%s, dry-run) error = %v", network, err)
			}

			if enableResult.UnitName == "" {
				t.Error("EnableService dry-run should return UnitName")
			}

			if enableResult.Enabled {
				t.Error("EnableService dry-run should not set Enabled")
			}

			if enableResult.Started {
				t.Error("EnableService dry-run should not set Started")
			}

			// Test disable dry-run
			disableResult, err := DisableService(network, true)
			if err != nil {
				t.Errorf("DisableService(%s, dry-run) error = %v", network, err)
			}

			if disableResult.UnitName == "" {
				t.Error("DisableService dry-run should return UnitName")
			}

			if disableResult.Stopped {
				t.Error("DisableService dry-run should not set Stopped")
			}

			if disableResult.Disabled {
				t.Error("DisableService dry-run should not set Disabled")
			}
		})
	}
}

// TestConfigSaveLoadFlow tests the config save/load dry-run workflow.
func TestConfigSaveLoadFlow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mesh-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	networks := []core.NetworkName{
		core.NetworkLocalnet,
		core.NetworkSprintnet,
		core.NetworkTestnet,
		core.NetworkMainnet,
	}

	for _, network := range networks {
		t.Run(string(network), func(t *testing.T) {
			cfg := DefaultConfig(network)

			// Test dry-run save
			content, err := SaveConfig(tmpDir, network, cfg, true)
			if err != nil {
				t.Errorf("SaveConfig(%s, dry-run) error = %v", network, err)
			}

			if content == "" {
				t.Error("SaveConfig dry-run should return content")
			}

			// Verify dry-run didn't create file
			if ConfigExists(tmpDir, network) {
				t.Error("SaveConfig dry-run should not create file")
			}

			// Verify content is valid JSON
			var parsed Config
			if err := json.Unmarshal([]byte(content), &parsed); err != nil {
				t.Errorf("SaveConfig dry-run content is not valid JSON: %v", err)
			}

			// Verify parsed config matches
			if parsed.ChainID != cfg.ChainID {
				t.Errorf("Parsed ChainID = %s, want %s", parsed.ChainID, cfg.ChainID)
			}

			// Actually save the config
			_, err = SaveConfig(tmpDir, network, cfg, false)
			if err != nil {
				t.Errorf("SaveConfig(%s) error = %v", network, err)
			}

			// Verify file was created
			if !ConfigExists(tmpDir, network) {
				t.Error("SaveConfig should create file")
			}

			// Load and verify
			loaded, err := LoadConfig(tmpDir, network)
			if err != nil {
				t.Errorf("LoadConfig(%s) error = %v", network, err)
			}

			if loaded.ChainID != cfg.ChainID {
				t.Errorf("Loaded ChainID = %s, want %s", loaded.ChainID, cfg.ChainID)
			}

			if loaded.Network != cfg.Network {
				t.Errorf("Loaded Network = %s, want %s", loaded.Network, cfg.Network)
			}
		})
	}
}

// TestSystemdUnitGenerationFlow tests the systemd unit generation workflow.
func TestSystemdUnitGenerationFlow(t *testing.T) {
	networks := []struct {
		network     string
		networkName core.NetworkName
	}{
		{"Localnet", core.NetworkLocalnet},
		{"Sprintnet", core.NetworkSprintnet},
		{"Testnet", core.NetworkTestnet},
		{"Mainnet", core.NetworkMainnet},
	}

	for _, n := range networks {
		t.Run(n.network, func(t *testing.T) {
			cfg := DefaultSystemdConfig(n.network, "monod", "/home/monod", n.networkName)

			// Test dry-run write
			unitPath, content, err := WriteSystemdUnit(cfg, true)
			if err != nil {
				t.Errorf("WriteSystemdUnit(%s, dry-run) error = %v", n.network, err)
			}

			if unitPath == "" {
				t.Error("WriteSystemdUnit dry-run should return unitPath")
			}

			if content == "" {
				t.Error("WriteSystemdUnit dry-run should return content")
			}

			// Verify content contains required sections
			requiredParts := []string{
				"[Unit]",
				"[Service]",
				"[Install]",
				"User=monod",
				"Restart=on-failure",
			}

			for _, part := range requiredParts {
				if !containsString(content, part) {
					t.Errorf("WriteSystemdUnit content missing %q", part)
				}
			}
		})
	}
}

// TestHealthCheckFlow tests the health check workflow.
func TestHealthCheckFlow(t *testing.T) {
	// Create test server with /health endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/network/list":
			if r.Method == "POST" {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"network_identifiers": []map[string]string{
						{"blockchain": "monolythium", "network": "sprintnet"},
					},
				})
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	hc := NewHealthChecker()
	ctx := context.Background()

	// Test health endpoint check
	status := hc.Check(ctx, server.URL)

	if !status.Healthy {
		t.Error("HealthCheck should return Healthy=true for working server")
	}

	if status.Method != "health_endpoint" {
		t.Errorf("HealthCheck Method = %s, want health_endpoint", status.Method)
	}

	if status.ResponseTime < 0 {
		t.Error("HealthCheck ResponseTime should be non-negative")
	}

	// Verify details are populated
	if status.Details == nil {
		t.Error("HealthCheck should populate Details from /health response")
	}
}

// TestUninstallDryRunFlow tests the uninstall dry-run workflow.
func TestUninstallDryRunFlow(t *testing.T) {
	err := Uninstall(false, true)
	if err != nil {
		t.Errorf("Uninstall(dry-run) error = %v", err)
	}

	err = Uninstall(true, true)
	if err != nil {
		t.Errorf("Uninstall(system, dry-run) error = %v", err)
	}
}

// TestDefaultConfigNetworks tests that default configs are created correctly for all networks.
func TestDefaultConfigNetworks(t *testing.T) {
	networks := []struct {
		networkName   core.NetworkName
		expectedChain string
		expectedPort  int
	}{
		{core.NetworkLocalnet, "mono-local-1", 8080},
		{core.NetworkSprintnet, "mono-sprint-1", 8081},
		{core.NetworkTestnet, "mono-test-1", 8082},
		{core.NetworkMainnet, "mono-1", 8083},
	}

	for _, n := range networks {
		t.Run(string(n.networkName), func(t *testing.T) {
			cfg := DefaultConfig(n.networkName)

			if cfg.ChainID != n.expectedChain {
				t.Errorf("DefaultConfig(%s) ChainID = %s, want %s", n.networkName, cfg.ChainID, n.expectedChain)
			}

			// Check listen address port
			if cfg.ListenAddress != "0.0.0.0:"+itoa(n.expectedPort) {
				t.Errorf("DefaultConfig(%s) ListenAddress = %s, want 0.0.0.0:%d", n.networkName, cfg.ListenAddress, n.expectedPort)
			}
		})
	}
}

// Helper functions

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

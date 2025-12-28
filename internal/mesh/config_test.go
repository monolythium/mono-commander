package mesh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/monolythium/mono-commander/internal/core"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig(core.NetworkSprintnet)

	if cfg.ChainID != "mono-sprint-1" {
		t.Errorf("DefaultConfig() ChainID = %s, want mono-sprint-1", cfg.ChainID)
	}

	if cfg.Network != "Sprintnet" {
		t.Errorf("DefaultConfig() Network = %s, want Sprintnet", cfg.Network)
	}

	if cfg.NodeRPCURL != "http://localhost:26657" {
		t.Errorf("DefaultConfig() NodeRPCURL = %s, want http://localhost:26657", cfg.NodeRPCURL)
	}

	if cfg.NodeGRPCAddress != "localhost:9090" {
		t.Errorf("DefaultConfig() NodeGRPCAddress = %s, want localhost:9090", cfg.NodeGRPCAddress)
	}

	if cfg.ListenAddress != "0.0.0.0:8081" {
		t.Errorf("DefaultConfig() ListenAddress = %s, want 0.0.0.0:8081", cfg.ListenAddress)
	}
}

func TestNetworkMeshPorts(t *testing.T) {
	tests := []struct {
		network core.NetworkName
		port    int
	}{
		{core.NetworkLocalnet, 8080},
		{core.NetworkSprintnet, 8081},
		{core.NetworkTestnet, 8082},
		{core.NetworkMainnet, 8083},
	}

	for _, tt := range tests {
		got := NetworkMeshPorts[tt.network]
		if got != tt.port {
			t.Errorf("NetworkMeshPorts[%s] = %d, want %d", tt.network, got, tt.port)
		}
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     DefaultConfig(core.NetworkLocalnet),
			wantErr: false,
		},
		{
			name:    "missing chain_id",
			cfg:     &Config{Network: "test", NodeRPCURL: "http://localhost:26657", NodeGRPCAddress: "localhost:9090", ListenAddress: "0.0.0.0:8080"},
			wantErr: true,
		},
		{
			name:    "missing network",
			cfg:     &Config{ChainID: "test", NodeRPCURL: "http://localhost:26657", NodeGRPCAddress: "localhost:9090", ListenAddress: "0.0.0.0:8080"},
			wantErr: true,
		},
		{
			name:    "missing node_rpc_url",
			cfg:     &Config{ChainID: "test", Network: "test", NodeGRPCAddress: "localhost:9090", ListenAddress: "0.0.0.0:8080"},
			wantErr: true,
		},
		{
			name:    "missing node_grpc_address",
			cfg:     &Config{ChainID: "test", Network: "test", NodeRPCURL: "http://localhost:26657", ListenAddress: "0.0.0.0:8080"},
			wantErr: true,
		},
		{
			name:    "missing listen_address",
			cfg:     &Config{ChainID: "test", Network: "test", NodeRPCURL: "http://localhost:26657", NodeGRPCAddress: "localhost:9090"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigMerge(t *testing.T) {
	cfg := DefaultConfig(core.NetworkSprintnet)

	opts := MergeOptions{
		ListenAddress:   "127.0.0.1:9090",
		NodeRPCURL:      "http://custom:26657",
		NodeGRPCAddress: "custom:9090",
	}

	cfg.Merge(opts)

	if cfg.ListenAddress != "127.0.0.1:9090" {
		t.Errorf("Merge() ListenAddress = %s, want 127.0.0.1:9090", cfg.ListenAddress)
	}

	if cfg.NodeRPCURL != "http://custom:26657" {
		t.Errorf("Merge() NodeRPCURL = %s, want http://custom:26657", cfg.NodeRPCURL)
	}

	if cfg.NodeGRPCAddress != "custom:9090" {
		t.Errorf("Merge() NodeGRPCAddress = %s, want custom:9090", cfg.NodeGRPCAddress)
	}
}

func TestConfigPath(t *testing.T) {
	path := ConfigPath("/home/user", core.NetworkSprintnet)
	expected := "/home/user/.mono/Sprintnet/mesh-rosetta/config.json"

	if path != expected {
		t.Errorf("ConfigPath() = %s, want %s", path, expected)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mesh-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := DefaultConfig(core.NetworkLocalnet)

	// Save config
	content, err := SaveConfig(tmpDir, core.NetworkLocalnet, cfg, false)
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	if content == "" {
		t.Error("SaveConfig() returned empty content")
	}

	// Load config
	loaded, err := LoadConfig(tmpDir, core.NetworkLocalnet)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.ChainID != cfg.ChainID {
		t.Errorf("LoadConfig() ChainID = %s, want %s", loaded.ChainID, cfg.ChainID)
	}

	if loaded.Network != cfg.Network {
		t.Errorf("LoadConfig() Network = %s, want %s", loaded.Network, cfg.Network)
	}
}

func TestSaveConfigDryRun(t *testing.T) {
	cfg := DefaultConfig(core.NetworkLocalnet)

	// Dry run should not create files
	content, err := SaveConfig("/nonexistent", core.NetworkLocalnet, cfg, true)
	if err != nil {
		t.Fatalf("SaveConfig(dry-run) error = %v", err)
	}

	// Verify content is valid JSON
	var parsed Config
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Errorf("SaveConfig(dry-run) returned invalid JSON: %v", err)
	}
}

func TestConfigExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mesh-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Should not exist initially
	if ConfigExists(tmpDir, core.NetworkLocalnet) {
		t.Error("ConfigExists() should return false for non-existent config")
	}

	// Create config
	cfg := DefaultConfig(core.NetworkLocalnet)
	_, err = SaveConfig(tmpDir, core.NetworkLocalnet, cfg, false)
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	// Should exist now
	if !ConfigExists(tmpDir, core.NetworkLocalnet) {
		t.Error("ConfigExists() should return true after saving config")
	}
}

func TestBinaryInstallPath(t *testing.T) {
	userPath := BinaryInstallPath(false)
	if !filepath.IsAbs(userPath) {
		t.Errorf("BinaryInstallPath(false) should return absolute path, got %s", userPath)
	}
	if filepath.Base(userPath) != "mono-mesh-rosetta" {
		t.Errorf("BinaryInstallPath(false) should end with mono-mesh-rosetta, got %s", userPath)
	}

	systemPath := BinaryInstallPath(true)
	if systemPath != "/usr/local/bin/mono-mesh-rosetta" {
		t.Errorf("BinaryInstallPath(true) = %s, want /usr/local/bin/mono-mesh-rosetta", systemPath)
	}
}

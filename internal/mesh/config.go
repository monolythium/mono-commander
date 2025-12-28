// Package mesh provides Mesh/Rosetta API sidecar management for mono-commander.
package mesh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/monolythium/mono-commander/internal/core"
)

// DefaultMeshPort is the default port for the Mesh/Rosetta API.
const DefaultMeshPort = 8080

// DefaultNodeRPCPort is the default CometBFT RPC port.
const DefaultNodeRPCPort = 26657

// DefaultNodeGRPCPort is the default Cosmos gRPC port.
const DefaultNodeGRPCPort = 9090

// NetworkMeshPorts maps networks to their default Mesh/Rosetta API ports.
var NetworkMeshPorts = map[core.NetworkName]int{
	core.NetworkLocalnet:  8080,
	core.NetworkSprintnet: 8081,
	core.NetworkTestnet:   8082,
	core.NetworkMainnet:   8083,
}

// Config holds the Mesh/Rosetta sidecar configuration.
type Config struct {
	// ChainID is the chain identifier (e.g., "mono-sprint-1").
	ChainID string `json:"chain_id"`

	// Network is the canonical network name.
	Network string `json:"network"`

	// NodeRPCURL is the CometBFT RPC endpoint URL (e.g., "http://localhost:26657").
	NodeRPCURL string `json:"node_rpc_url"`

	// NodeGRPCAddress is the Cosmos gRPC address (e.g., "localhost:9090").
	NodeGRPCAddress string `json:"node_grpc_address"`

	// ListenAddress is the address the sidecar listens on (e.g., "0.0.0.0:8080").
	ListenAddress string `json:"listen_address"`

	// Offline indicates whether to run in offline mode (no node connection).
	Offline bool `json:"offline,omitempty"`

	// RetryCount is the number of retries for RPC calls.
	RetryCount int `json:"retry_count,omitempty"`
}

// DefaultConfig returns a default configuration for a given network.
func DefaultConfig(network core.NetworkName) *Config {
	netCfg, err := core.GetNetwork(network)
	chainID := ""
	if err == nil {
		chainID = netCfg.ChainID
	}

	port := DefaultMeshPort
	if p, ok := NetworkMeshPorts[network]; ok {
		port = p
	}

	return &Config{
		ChainID:         chainID,
		Network:         string(network),
		NodeRPCURL:      fmt.Sprintf("http://localhost:%d", DefaultNodeRPCPort),
		NodeGRPCAddress: fmt.Sprintf("localhost:%d", DefaultNodeGRPCPort),
		ListenAddress:   fmt.Sprintf("0.0.0.0:%d", port),
		RetryCount:      3,
	}
}

// ConfigPath returns the path to the config file for a given network and home.
func ConfigPath(home string, network core.NetworkName) string {
	return filepath.Join(home, ".mono", string(network), "mesh-rosetta", "config.json")
}

// ConfigDir returns the directory containing the config file.
func ConfigDir(home string, network core.NetworkName) string {
	return filepath.Join(home, ".mono", string(network), "mesh-rosetta")
}

// LoadConfig loads the configuration from the default path.
func LoadConfig(home string, network core.NetworkName) (*Config, error) {
	path := ConfigPath(home, network)
	return LoadConfigFromPath(path)
}

// LoadConfigFromPath loads the configuration from a specific path.
func LoadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to the default path.
func SaveConfig(home string, network core.NetworkName, cfg *Config, dryRun bool) (string, error) {
	path := ConfigPath(home, network)
	return SaveConfigToPath(path, cfg, dryRun)
}

// SaveConfigToPath saves the configuration to a specific path.
func SaveConfigToPath(path string, cfg *Config, dryRun bool) (string, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	if dryRun {
		return string(data), nil
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	return string(data), nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.ChainID == "" {
		return fmt.Errorf("chain_id is required")
	}
	if c.Network == "" {
		return fmt.Errorf("network is required")
	}
	if c.NodeRPCURL == "" {
		return fmt.Errorf("node_rpc_url is required")
	}
	if c.NodeGRPCAddress == "" {
		return fmt.Errorf("node_grpc_address is required")
	}
	if c.ListenAddress == "" {
		return fmt.Errorf("listen_address is required")
	}
	return nil
}

// MergeOptions merges CLI options into the config.
type MergeOptions struct {
	ListenAddress   string
	NodeRPCURL      string
	NodeGRPCAddress string
}

// Merge applies options to override config values.
func (c *Config) Merge(opts MergeOptions) {
	if opts.ListenAddress != "" {
		c.ListenAddress = opts.ListenAddress
	}
	if opts.NodeRPCURL != "" {
		c.NodeRPCURL = opts.NodeRPCURL
	}
	if opts.NodeGRPCAddress != "" {
		c.NodeGRPCAddress = opts.NodeGRPCAddress
	}
}

// BinaryInstallPath returns the recommended install path for the mesh binary.
// Prefers user-local install: ~/.local/bin/mono-mesh-rosetta
func BinaryInstallPath(useSystemPath bool) string {
	if useSystemPath {
		return "/usr/local/bin/mono-mesh-rosetta"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin", "mono-mesh-rosetta")
}

// ConfigExists checks if the config file exists.
func ConfigExists(home string, network core.NetworkName) bool {
	path := ConfigPath(home, network)
	_, err := os.Stat(path)
	return err == nil
}

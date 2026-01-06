package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// NetworksRepoURL is the base URL for the networks repository
	NetworksRepoURL = "https://raw.githubusercontent.com/monolythium/networks"
	// DefaultRef is the default git ref to fetch from
	DefaultRef = "main"
)

// NetworkConfig represents a network configuration from the networks repo
type NetworkConfig struct {
	NetworkName    string            `json:"network_name"`
	CosmosChainID  string            `json:"cosmos_chain_id"`
	EVMChainID     uint64            `json:"evm_chain_id"`
	EVMChainIDHex  string            `json:"evm_chain_id_hex"`
	GenesisURL     string            `json:"genesis_url"`
	GenesisSHA256  string            `json:"genesis_sha256"`
	Seeds          []string          `json:"seeds"`
	BootstrapPeers []string          `json:"bootstrap_peers"`
	RPCEndpoints   map[string]string `json:"rpc_endpoints"`
	PortScheme     map[string]int    `json:"port_scheme"`
	NetworkStatus  string            `json:"network_status"`
	ConfigVersion  string            `json:"config_version"`
	UpdatedAt      string            `json:"updated_at"`
}

// NetworkIndex represents the index of all available networks
type NetworkIndex struct {
	Networks  []string `json:"networks"`
	UpdatedAt string   `json:"updated_at"`
}

// FetchNetworkConfig fetches a network configuration from the networks repo
func FetchNetworkConfig(network, ref string) (*NetworkConfig, error) {
	if ref == "" {
		ref = DefaultRef
	}

	url := fmt.Sprintf("%s/%s/networks/%s.json", NetworksRepoURL, ref, network)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch network config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("network '%s' not found in networks repo", network)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch network config: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var config NetworkConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("failed to parse network config: %w", err)
	}

	return &config, nil
}

// FetchNetworkIndex fetches the index of all available networks
func FetchNetworkIndex(ref string) (*NetworkIndex, error) {
	if ref == "" {
		ref = DefaultRef
	}

	url := fmt.Sprintf("%s/%s/index.json", NetworksRepoURL, ref)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch network index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch network index: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var index NetworkIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("failed to parse network index: %w", err)
	}

	return &index, nil
}

// VerifyNetworkConfig validates the network configuration
func VerifyNetworkConfig(config *NetworkConfig) error {
	// Verify EVM chain ID matches expected values
	switch config.NetworkName {
	case "Sprintnet":
		if config.EVMChainID != 262146 {
			return fmt.Errorf("Sprintnet must have evm_chain_id=262146, got %d", config.EVMChainID)
		}
	case "Testnet":
		if config.EVMChainID != 262147 {
			return fmt.Errorf("Testnet must have evm_chain_id=262147, got %d", config.EVMChainID)
		}
	case "Mainnet":
		if config.EVMChainID != 262148 {
			return fmt.Errorf("Mainnet must have evm_chain_id=262148, got %d", config.EVMChainID)
		}
	case "Localnet":
		if config.EVMChainID != 262145 {
			return fmt.Errorf("Localnet must have evm_chain_id=262145, got %d", config.EVMChainID)
		}
	}

	// Verify genesis SHA256 format
	if len(config.GenesisSHA256) != 64 {
		return fmt.Errorf("invalid genesis SHA256 length: expected 64, got %d", len(config.GenesisSHA256))
	}

	return nil
}

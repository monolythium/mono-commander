// Package core provides the core logic for mono-commander.
// All TUI and CLI commands call functions in this package.
package core

import (
	"fmt"
	"strings"
)

// LocalnetEVMChainID is the EVM chain ID reserved for Localnet development.
// This value MUST NOT be used by any production network.
const LocalnetEVMChainID = 262145

// NetworkName represents a canonical network name.
type NetworkName string

const (
	NetworkLocalnet  NetworkName = "Localnet"
	NetworkSprintnet NetworkName = "Sprintnet"
	NetworkTestnet   NetworkName = "Testnet"
	NetworkMainnet   NetworkName = "Mainnet"
)

// Network holds configuration for a Monolythium network.
type Network struct {
	Name       NetworkName
	ChainID    string
	EVMChainID uint64
	SeedDNS    []string
	GenesisURL string
	PeersURL   string
}

// networks is the canonical registry of supported networks.
var networks = map[NetworkName]Network{
	NetworkLocalnet: {
		Name:       NetworkLocalnet,
		ChainID:    "mono-local-1",
		EVMChainID: 262145,     // 0x40001
		SeedDNS:    []string{}, // Localnet has no DNS seeds
		GenesisURL: "",
		PeersURL:   "",
	},
	NetworkSprintnet: {
		Name:       NetworkSprintnet,
		ChainID:    "mono-sprint-1",
		EVMChainID: 262146, // 0x40002
		SeedDNS: []string{
			"seed1.sprintnet.mononodes.xyz",
			"seed2.sprintnet.mononodes.xyz",
			"seed3.sprintnet.mononodes.xyz",
		},
		GenesisURL: "https://raw.githubusercontent.com/monolythium/networks/main/sprintnet/genesis.json",
		PeersURL:   "https://raw.githubusercontent.com/monolythium/networks/main/networks/sprintnet.json",
	},
	NetworkTestnet: {
		Name:       NetworkTestnet,
		ChainID:    "mono-test-1",
		EVMChainID: 262147, // 0x40003
		SeedDNS: []string{
			"seed1.testnet.mononodes.xyz",
			"seed2.testnet.mononodes.xyz",
			"seed3.testnet.mononodes.xyz",
		},
		GenesisURL: "https://raw.githubusercontent.com/monolythium/mono-core-peers/prod/networks/testnet/genesis.json",
		PeersURL:   "https://raw.githubusercontent.com/monolythium/mono-core-peers/prod/networks/testnet/peers.json",
	},
	NetworkMainnet: {
		Name:       NetworkMainnet,
		ChainID:    "mono-1",
		EVMChainID: 262148, // 0x40004
		SeedDNS: []string{
			"seed1.mainnet.mononodes.xyz",
			"seed2.mainnet.mononodes.xyz",
			"seed3.mainnet.mononodes.xyz",
		},
		GenesisURL: "https://raw.githubusercontent.com/monolythium/mono-core-peers/prod/networks/mainnet/genesis.json",
		PeersURL:   "https://raw.githubusercontent.com/monolythium/mono-core-peers/prod/networks/mainnet/peers.json",
	},
}

// GetNetwork returns the network configuration for the given name.
func GetNetwork(name NetworkName) (Network, error) {
	n, ok := networks[name]
	if !ok {
		return Network{}, fmt.Errorf("unknown network: %s", name)
	}
	return n, nil
}

// GetNetworkByChainID returns the network configuration for the given chain ID.
func GetNetworkByChainID(chainID string) (Network, error) {
	for _, n := range networks {
		if n.ChainID == chainID {
			return n, nil
		}
	}
	return Network{}, fmt.Errorf("unknown chain ID: %s", chainID)
}

// ListNetworks returns all supported networks.
func ListNetworks() []Network {
	return []Network{
		networks[NetworkLocalnet],
		networks[NetworkSprintnet],
		networks[NetworkTestnet],
		networks[NetworkMainnet],
	}
}

// ParseNetworkName parses a string into a NetworkName.
func ParseNetworkName(s string) (NetworkName, error) {
	lower := strings.ToLower(s)
	switch lower {
	case "localnet":
		return NetworkLocalnet, nil
	case "sprintnet":
		return NetworkSprintnet, nil
	case "testnet":
		return NetworkTestnet, nil
	case "mainnet":
		return NetworkMainnet, nil
	default:
		return "", fmt.Errorf("unknown network: %s (valid: Localnet, Sprintnet, Testnet, Mainnet)", s)
	}
}

// EVMChainIDHex returns the EVM chain ID as a hex string.
func (n Network) EVMChainIDHex() string {
	return fmt.Sprintf("0x%x", n.EVMChainID)
}

// SeedString returns the seeds as a comma-separated string for config.toml.
// Format: node_id@host:port (port defaults to 26656)
func (n Network) SeedString(port int) string {
	if len(n.SeedDNS) == 0 {
		return ""
	}
	if port == 0 {
		port = 26656
	}
	seeds := make([]string, len(n.SeedDNS))
	for i, dns := range n.SeedDNS {
		// Note: In production, we'd need to resolve the node ID from the seed
		// For now, we use placeholder format that requires resolution
		seeds[i] = fmt.Sprintf("%s:%d", dns, port)
	}
	return strings.Join(seeds, ",")
}

// GetNetworkFromCanonical fetches network configuration from the canonical
// networks repository. For Localnet, it returns embedded defaults.
// For all other networks, it fetches from the networks repo with cache fallback.
//
// This is the preferred method for getting network configuration as it
// ensures consistency with the canonical source of truth.
func GetNetworkFromCanonical(name NetworkName, ref string) (Network, error) {
	// Localnet always uses embedded defaults - it's a local development network
	if name == NetworkLocalnet {
		return networks[NetworkLocalnet], nil
	}

	// Map NetworkName to lowercase for fetch
	networkID := strings.ToLower(string(name))

	// Fetch from canonical source with cache fallback
	config, err := GetNetworkConfigWithCache(networkID, ref)
	if err != nil {
		// Fall back to embedded config if fetch fails
		n, ok := networks[name]
		if !ok {
			return Network{}, fmt.Errorf("network %s not found and fetch failed: %w", name, err)
		}
		return n, nil
	}

	// Validate the fetched config
	if err := VerifyNetworkConfig(config); err != nil {
		return Network{}, fmt.Errorf("canonical config validation failed: %w", err)
	}

	// Check for Localnet EVM chain ID leak (CRITICAL)
	if err := ValidateNotLocalnetLeak(config); err != nil {
		return Network{}, err
	}

	// Convert NetworkConfig to Network struct
	return NetworkConfigToNetwork(config), nil
}

// ValidateNotLocalnetLeak ensures a non-Localnet network doesn't use Localnet's EVM chain ID.
// This prevents the critical bug where production networks accidentally use 262145.
func ValidateNotLocalnetLeak(config *NetworkConfig) error {
	if config.NetworkName != "Localnet" && config.EVMChainID == LocalnetEVMChainID {
		return fmt.Errorf("FATAL: Localnet EVM chain ID (%d) detected for %s. "+
			"This is a configuration error that will cause consensus failures. "+
			"Expected EVM chain ID for %s is NOT %d",
			LocalnetEVMChainID, config.NetworkName, config.NetworkName, LocalnetEVMChainID)
	}
	return nil
}

// NetworkConfigToNetwork converts a NetworkConfig (from networks repo) to a Network struct.
// This provides backwards compatibility with existing code that uses the Network struct.
func NetworkConfigToNetwork(config *NetworkConfig) Network {
	return Network{
		Name:       NetworkName(config.NetworkName),
		ChainID:    config.CosmosChainID,
		EVMChainID: config.EVMChainID,
		SeedDNS:    []string{}, // Seeds are now in node_id@host:port format in config.Seeds
		GenesisURL: config.GenesisURL,
		PeersURL:   "", // Peers are now embedded in the NetworkConfig
	}
}

// GetNetworkConfigAsDriftConfig converts a NetworkConfig to a DriftConfig for drift detection.
func GetNetworkConfigAsDriftConfig(config *NetworkConfig) *DriftConfig {
	return &DriftConfig{
		CosmosChainID:  config.CosmosChainID,
		EVMChainID:     config.EVMChainID,
		Seeds:          config.Seeds,
		BootstrapPeers: config.BootstrapPeers,
	}
}

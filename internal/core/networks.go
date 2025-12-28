// Package core provides the core logic for mono-commander.
// All TUI and CLI commands call functions in this package.
package core

import (
	"fmt"
	"strings"
)

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
		PeersURL:   "https://raw.githubusercontent.com/monolythium/networks/main/sprintnet/peers.json",
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
		GenesisURL: "https://raw.githubusercontent.com/monolythium/networks/main/testnet/genesis.json",
		PeersURL:   "https://raw.githubusercontent.com/monolythium/networks/main/testnet/peers.json",
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
		GenesisURL: "https://raw.githubusercontent.com/monolythium/networks/main/mainnet/genesis.json",
		PeersURL:   "https://raw.githubusercontent.com/monolythium/networks/main/mainnet/peers.json",
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

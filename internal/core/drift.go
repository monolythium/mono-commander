package core

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// DriftSeverity indicates how critical a drift is
type DriftSeverity string

const (
	SeverityCritical DriftSeverity = "CRITICAL"
	SeverityWarning  DriftSeverity = "WARNING"
	SeverityInfo     DriftSeverity = "INFO"
)

// DriftResult represents a single configuration drift
type DriftResult struct {
	Field    string        // "chain-id", "evm-chain-id", "seeds", etc.
	Expected string        // Expected value from canonical config
	Actual   string        // Actual value on disk
	File     string        // "client.toml", "app.toml", "config.toml"
	Severity DriftSeverity // How critical this drift is
}

// DriftConfig holds the expected configuration for drift detection.
// This combines Network settings with peers registry data.
type DriftConfig struct {
	CosmosChainID  string
	EVMChainID     uint64
	Seeds          []string // In node_id@host:port format
	BootstrapPeers []string // In node_id@host:port format
}

// DetectDrift compares on-disk configuration against canonical config
func DetectDrift(home string, config *DriftConfig) ([]DriftResult, error) {
	var drifts []DriftResult

	configDir := filepath.Join(home, "config")

	// Check client.toml chain-id
	clientPath := filepath.Join(configDir, "client.toml")
	chainID, err := GetConfigValue(clientPath, "", "chain-id")
	if err == nil {
		// Remove quotes if present
		chainID = strings.Trim(chainID, "\"")
		if chainID != config.CosmosChainID {
			drifts = append(drifts, DriftResult{
				Field:    "chain-id",
				Expected: config.CosmosChainID,
				Actual:   chainID,
				File:     "client.toml",
				Severity: SeverityCritical,
			})
		}
	}

	// Check app.toml evm-chain-id
	appPath := filepath.Join(configDir, "app.toml")
	evmChainIDStr, err := GetConfigValue(appPath, "evm", "evm-chain-id")
	if err == nil {
		evmChainID, _ := strconv.ParseUint(evmChainIDStr, 10, 64)
		if evmChainID != config.EVMChainID {
			drifts = append(drifts, DriftResult{
				Field:    "evm-chain-id",
				Expected: fmt.Sprintf("%d", config.EVMChainID),
				Actual:   evmChainIDStr,
				File:     "app.toml",
				Severity: SeverityCritical,
			})
		}
	}

	// Check config.toml seeds
	configPath := filepath.Join(configDir, "config.toml")
	seeds, err := GetConfigValue(configPath, "p2p", "seeds")
	if err == nil {
		seeds = strings.Trim(seeds, "\"")
		expectedSeeds := strings.Join(config.Seeds, ",")
		if seeds != expectedSeeds && len(config.Seeds) > 0 {
			drifts = append(drifts, DriftResult{
				Field:    "seeds",
				Expected: expectedSeeds,
				Actual:   seeds,
				File:     "config.toml",
				Severity: SeverityWarning,
			})
		}
	}

	// Check config.toml persistent_peers (for bootstrap mode)
	peers, err := GetConfigValue(configPath, "p2p", "persistent_peers")
	if err == nil {
		peers = strings.Trim(peers, "\"")
		expectedPeers := strings.Join(config.BootstrapPeers, ",")
		if peers != expectedPeers && len(config.BootstrapPeers) > 0 {
			drifts = append(drifts, DriftResult{
				Field:    "persistent_peers",
				Expected: expectedPeers,
				Actual:   peers,
				File:     "config.toml",
				Severity: SeverityWarning,
			})
		}
	}

	return drifts, nil
}

// HasCriticalDrift returns true if any drift is critical
func HasCriticalDrift(drifts []DriftResult) bool {
	for _, d := range drifts {
		if d.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// FormatDriftReport formats drift results for display
func FormatDriftReport(drifts []DriftResult) string {
	if len(drifts) == 0 {
		return "No drift detected. Configuration matches canonical source."
	}

	var sb strings.Builder
	sb.WriteString("DRIFT DETECTED:\n")

	for _, d := range drifts {
		sb.WriteString(fmt.Sprintf("  [%s] %s %s: expected '%s', got '%s'\n",
			d.Severity, d.File, d.Field, d.Expected, d.Actual))
	}

	if HasCriticalDrift(drifts) {
		sb.WriteString("\nRun: monoctl repair --network <network> to fix critical issues\n")
	}

	return sb.String()
}

// ParsePeer parses a peer string in format "nodeid@host:port".
// This is a public wrapper for the internal parsePeerString function.
func ParsePeer(s string) (Peer, error) {
	matches := peerStringRegex.FindStringSubmatch(s)
	if matches == nil {
		return Peer{}, fmt.Errorf("invalid peer string format: %s (expected nodeid@host:port)", s)
	}

	port, err := strconv.Atoi(matches[3])
	if err != nil {
		return Peer{}, fmt.Errorf("invalid port in peer string: %s", matches[3])
	}

	return Peer{
		NodeID:  strings.ToLower(matches[1]),
		Address: matches[2],
		Port:    port,
	}, nil
}

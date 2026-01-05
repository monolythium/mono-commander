package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
)

// SyncStrategy defines how the node should sync.
type SyncStrategy string

const (
	// SyncStrategyDefault uses standard peer discovery (seeds + pex).
	SyncStrategyDefault SyncStrategy = "default"
	// SyncStrategyBootstrap uses bootstrap_peers with pex=false for deterministic genesis sync.
	SyncStrategyBootstrap SyncStrategy = "bootstrap"
	// SyncStrategyStateSync uses state sync for fast sync (trust but verify).
	SyncStrategyStateSync SyncStrategy = "statesync"
)

// JoinOptions contains options for the join operation.
type JoinOptions struct {
	Network      NetworkName
	Home         string
	GenesisURL   string
	GenesisSHA   string
	PeersURL     string
	DryRun       bool
	Logger       *slog.Logger
	SyncStrategy SyncStrategy // default, bootstrap, statesync
	ClearAddrbook bool        // Clear addrbook.json on bootstrap mode
}

// JoinResult contains the results of a join operation.
type JoinResult struct {
	GenesisPath     string
	ConfigPatchPath string
	ConfigPatch     string
	ChainID         string
	Success         bool
	Steps           []JoinStep
}

// JoinStep represents a step in the join process.
type JoinStep struct {
	Name    string
	Status  string // "pending", "success", "failed", "skipped"
	Message string
}

// Join executes the join flow to set up a node for a network.
func Join(opts JoinOptions, fetcher Fetcher) (*JoinResult, error) {
	result := &JoinResult{
		Steps: make([]JoinStep, 0),
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Get network config
	network, err := GetNetwork(opts.Network)
	if err != nil {
		return nil, fmt.Errorf("invalid network: %w", err)
	}

	// Use network defaults if not specified
	genesisURL := opts.GenesisURL
	if genesisURL == "" {
		genesisURL = network.GenesisURL
	}

	if genesisURL == "" {
		return nil, fmt.Errorf("genesis URL required for network %s", opts.Network)
	}

	// Step 1: Download genesis
	logger.Info("downloading genesis", "url", genesisURL)
	result.Steps = append(result.Steps, JoinStep{Name: "Download genesis", Status: "pending"})

	genesisData, err := fetcher.Fetch(genesisURL)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		return result, fmt.Errorf("failed to download genesis: %w", err)
	}
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 2: Validate genesis JSON
	logger.Info("validating genesis")
	result.Steps = append(result.Steps, JoinStep{Name: "Validate genesis", Status: "pending"})

	chainID, err := ValidateGenesisData(genesisData)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		return result, fmt.Errorf("invalid genesis: %w", err)
	}

	if chainID != network.ChainID {
		result.Steps[len(result.Steps)-1].Status = "failed"
		msg := fmt.Sprintf("chain_id mismatch: expected %s, got %s - wrong genesis for %s, do NOT proceed", network.ChainID, chainID, network.Name)
		result.Steps[len(result.Steps)-1].Message = msg
		return result, fmt.Errorf("FATAL: wrong genesis for %s - expected chain_id %s, got %s. Use canonical genesis from mono-core-peers/prod", network.Name, network.ChainID, chainID)
	}
	result.ChainID = chainID
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 3: Download peers to get genesis SHA256
	peersURL := opts.PeersURL
	if peersURL == "" {
		peersURL = network.PeersURL
	}

	var genesisSHA string
	var seeds []Peer
	var persistentPeers []Peer
	var bootstrapPeers []Peer
	var pexEnabled = true // Default: pex is enabled

	if peersURL != "" {
		logger.Info("downloading peers", "url", peersURL)
		result.Steps = append(result.Steps, JoinStep{Name: "Download peers", Status: "pending"})

		peersData, err := fetcher.Fetch(peersURL)
		if err != nil {
			// Non-fatal: log warning and continue
			result.Steps[len(result.Steps)-1].Status = "skipped"
			result.Steps[len(result.Steps)-1].Message = err.Error()
			logger.Warn("failed to download peers, continuing without", "error", err)
		} else {
			reg, err := ParsePeersRegistry(peersData)
			if err != nil {
				result.Steps[len(result.Steps)-1].Status = "skipped"
				result.Steps[len(result.Steps)-1].Message = err.Error()
				logger.Warn("failed to parse peers, continuing without", "error", err)
			} else {
				if reg.ChainID != network.ChainID {
					result.Steps[len(result.Steps)-1].Status = "skipped"
					msg := fmt.Sprintf("chain_id mismatch: expected %s, got %s", network.ChainID, reg.ChainID)
					result.Steps[len(result.Steps)-1].Message = msg
					logger.Warn("peers registry validation failed", "error", msg)
				} else {
					// Use seeds from registry (all must be node_id@host:port format)
					seeds = reg.Seeds
					persistentPeers = MergePeers(reg.Peers, reg.PersistentPeers)
					bootstrapPeers = reg.BootstrapPeers
					genesisSHA = reg.GenesisSHA
					result.Steps[len(result.Steps)-1].Status = "success"
					result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("%d seeds, %d peers, %d bootstrap", len(seeds), len(persistentPeers), len(bootstrapPeers))
				}
			}
		}
	}

	// Handle bootstrap sync strategy
	if opts.SyncStrategy == SyncStrategyBootstrap {
		logger.Info("bootstrap mode: using bootstrap_peers with pex=false")
		if len(bootstrapPeers) == 0 {
			// Fall back to persistent_peers if no bootstrap_peers defined
			if len(persistentPeers) > 0 {
				logger.Warn("no bootstrap_peers in registry, falling back to persistent_peers")
				bootstrapPeers = persistentPeers
			} else {
				return result, fmt.Errorf("bootstrap mode requires bootstrap_peers in registry (none found)")
			}
		}
		// In bootstrap mode:
		// - Use bootstrap_peers as persistent_peers
		// - Clear seeds (don't rely on seed discovery)
		// - Disable pex
		persistentPeers = bootstrapPeers
		seeds = nil // No seeds in bootstrap mode
		pexEnabled = false
		result.Steps = append(result.Steps, JoinStep{
			Name:    "Configure bootstrap mode",
			Status:  "success",
			Message: fmt.Sprintf("pex=false, %d bootstrap peers", len(bootstrapPeers)),
		})
	}

	// Step 4: Verify SHA256 (use opts.GenesisSHA if provided, otherwise use from peers.json)
	expectedSHA := opts.GenesisSHA
	if expectedSHA == "" {
		expectedSHA = genesisSHA
	}

	if expectedSHA != "" {
		logger.Info("verifying genesis SHA256", "expected", expectedSHA)
		result.Steps = append(result.Steps, JoinStep{Name: "Verify SHA256", Status: "pending"})

		hash := sha256.Sum256(genesisData)
		actual := hex.EncodeToString(hash[:])

		if actual != expectedSHA {
			result.Steps[len(result.Steps)-1].Status = "failed"
			msg := fmt.Sprintf("SHA256 mismatch: expected %s, got %s - wrong genesis, do NOT proceed", expectedSHA, actual)
			result.Steps[len(result.Steps)-1].Message = msg
			return result, fmt.Errorf("FATAL: wrong genesis for %s - SHA256 mismatch (expected %s, got %s). Use canonical genesis from mono-core-peers/prod", opts.Network, expectedSHA, actual)
		}
		result.Steps[len(result.Steps)-1].Status = "success"
	}

	// Step 5: Write genesis file
	logger.Info("writing genesis", "home", opts.Home, "dry_run", opts.DryRun)
	result.Steps = append(result.Steps, JoinStep{Name: "Write genesis", Status: "pending"})

	genesisPath, err := WriteGenesis(opts.Home, genesisData, opts.DryRun)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		return result, fmt.Errorf("failed to write genesis: %w", err)
	}
	result.GenesisPath = genesisPath
	if opts.DryRun {
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = "(dry-run)"
	} else {
		result.Steps[len(result.Steps)-1].Status = "success"
	}

	// Step 6: Clear addrbook if in bootstrap mode
	if opts.SyncStrategy == SyncStrategyBootstrap || opts.ClearAddrbook {
		logger.Info("clearing addrbook.json")
		result.Steps = append(result.Steps, JoinStep{Name: "Clear addrbook", Status: "pending"})
		if err := ClearAddrbook(opts.Home, opts.DryRun); err != nil {
			result.Steps[len(result.Steps)-1].Status = "skipped"
			result.Steps[len(result.Steps)-1].Message = err.Error()
		} else {
			result.Steps[len(result.Steps)-1].Status = "success"
			if opts.DryRun {
				result.Steps[len(result.Steps)-1].Message = "(dry-run)"
			}
		}
	}

	// Step 7: Generate config patch
	logger.Info("generating config patch")
	result.Steps = append(result.Steps, JoinStep{Name: "Generate config", Status: "pending"})

	var patch *ConfigPatch
	if pexEnabled {
		patch = GenerateConfigPatch(seeds, persistentPeers)
	} else {
		patch = GenerateBootstrapConfigPatch(persistentPeers)
	}

	patchPath, patchContent, err := WriteConfigPatch(opts.Home, patch, opts.DryRun)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		return result, fmt.Errorf("failed to write config patch: %w", err)
	}
	result.ConfigPatchPath = patchPath
	result.ConfigPatch = patchContent
	result.Steps[len(result.Steps)-1].Status = "success"

	result.Success = true
	return result, nil
}

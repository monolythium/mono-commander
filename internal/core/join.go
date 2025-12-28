package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
)

// JoinOptions contains options for the join operation.
type JoinOptions struct {
	Network    NetworkName
	Home       string
	GenesisURL string
	GenesisSHA string
	PeersURL   string
	DryRun     bool
	Logger     *slog.Logger
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
		msg := fmt.Sprintf("chain_id mismatch: expected %s, got %s", network.ChainID, chainID)
		result.Steps[len(result.Steps)-1].Message = msg
		return result, fmt.Errorf("chain_id mismatch: expected %s, got %s", network.ChainID, chainID)
	}
	result.ChainID = chainID
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 3: Verify SHA256 if provided
	if opts.GenesisSHA != "" {
		logger.Info("verifying genesis SHA256", "expected", opts.GenesisSHA)
		result.Steps = append(result.Steps, JoinStep{Name: "Verify SHA256", Status: "pending"})

		hash := sha256.Sum256(genesisData)
		actual := hex.EncodeToString(hash[:])

		if actual != opts.GenesisSHA {
			result.Steps[len(result.Steps)-1].Status = "failed"
			msg := fmt.Sprintf("SHA256 mismatch: expected %s, got %s", opts.GenesisSHA, actual)
			result.Steps[len(result.Steps)-1].Message = msg
			return result, fmt.Errorf("SHA256 mismatch: expected %s, got %s", opts.GenesisSHA, actual)
		}
		result.Steps[len(result.Steps)-1].Status = "success"
	}

	// Step 4: Write genesis file
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

	// Step 5: Download and process peers if URL provided
	peersURL := opts.PeersURL
	if peersURL == "" {
		peersURL = network.PeersURL
	}

	var persistentPeers []Peer
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
				if err := ValidatePeersRegistry(reg, network.ChainID, opts.GenesisSHA); err != nil {
					result.Steps[len(result.Steps)-1].Status = "skipped"
					result.Steps[len(result.Steps)-1].Message = err.Error()
					logger.Warn("peers registry validation failed", "error", err)
				} else {
					persistentPeers = MergePeers(reg.Peers, reg.PersistentPeers)
					result.Steps[len(result.Steps)-1].Status = "success"
					result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("%d peers", len(persistentPeers))
				}
			}
		}
	}

	// Step 6: Generate config patch
	logger.Info("generating config patch")
	result.Steps = append(result.Steps, JoinStep{Name: "Generate config", Status: "pending"})

	patch := GenerateConfigPatch(network, persistentPeers)
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

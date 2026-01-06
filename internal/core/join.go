package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/monolythium/mono-commander/internal/net"
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
	Network       NetworkName
	Home          string
	GenesisURL    string
	GenesisSHA    string
	PeersURL      string
	DryRun        bool
	Logger        *slog.Logger
	SyncStrategy  SyncStrategy // default, bootstrap, statesync
	ClearAddrbook bool         // Clear addrbook.json on bootstrap mode
	Moniker       string       // Node moniker (auto-generated if empty)
	MonodPath     string       // Path to monod binary (auto-detected if empty)
}

// JoinResult contains the results of a join operation.
type JoinResult struct {
	GenesisPath     string
	ConfigPatchPath string
	ConfigPatch     string
	ChainID         string
	NodeID          string
	Success         bool
	Steps           []JoinStep
	Initialized     bool // True if node was initialized during this run
}

// JoinStep represents a step in the join process.
type JoinStep struct {
	Name    string
	Status  string // "pending", "success", "failed", "skipped"
	Message string
}

// IsNodeHomeInitialized checks if the node home directory is initialized.
// A node is considered initialized if config.toml exists.
func IsNodeHomeInitialized(home string) bool {
	configPath := filepath.Join(home, "config", "config.toml")
	_, err := os.Stat(configPath)
	return err == nil
}

// PreflightError represents an error detected during preflight checks.
type PreflightError struct {
	Type    string // "stale_data", "chain_id_mismatch", "genesis_mismatch"
	Message string
	Details string
}

func (e *PreflightError) Error() string {
	return e.Message
}

// HasStaleData checks if the data directory contains stale blockchain data.
// Returns true if ANY of the database directories exist (state.db, application.db, etc.)
// This catches partial InitChain failures that leave corrupt state.
func HasStaleData(home string) bool {
	dataPath := filepath.Join(home, "data")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return false
	}

	// Check for key database directories that indicate blockchain data.
	// If ANY of these exist, the data directory is dirty and must be reset.
	// This is critical: monod creates these during InitChain, and if InitChain
	// fails partway through, these directories contain corrupt/partial state
	// that will cause "invalid chain-id on InitChain" errors on restart.
	dbDirs := []string{
		"state.db",
		"application.db",
		"blockstore.db",
		"tx_index.db",
		"evidence.db",
		"snapshots",
	}

	for _, db := range dbDirs {
		dbPath := filepath.Join(dataPath, db)
		if _, err := os.Stat(dbPath); err == nil {
			// Directory exists - data is dirty
			return true
		}
	}

	return false
}

// GetExistingChainID attempts to read the chain-id from the existing genesis.json.
// Returns empty string if genesis doesn't exist or can't be parsed.
func GetExistingChainID(home string) string {
	genesisPath := filepath.Join(home, "config", "genesis.json")
	data, err := os.ReadFile(genesisPath)
	if err != nil {
		return ""
	}

	chainID, err := ValidateGenesisData(data)
	if err != nil {
		return ""
	}

	return chainID
}

// PreflightCheck runs preflight checks before joining a network.
// It detects conditions that would cause the node to fail on startup.
//
// Note on genesis SHA: Genesis SHA mismatch with node-local files may occur due to
// SDK formatting differences (e.g., initial_height as string vs integer).
// Consensus equivalence is determined by AppHash, not file SHA.
func PreflightCheck(home string, expectedChainID string) *PreflightError {
	// Check 1: Detect dirty data directory (leftover from failed InitChain)
	// This is the PRIMARY check - if ANY *.db directories exist, the node will fail.
	// This must run BEFORE checking initialization status because a partial init
	// leaves databases but may not complete config file creation.
	if HasStaleData(home) {
		return &PreflightError{
			Type:    "dirty_data",
			Message: "Detected leftover blockchain state from a failed initialization",
			Details: fmt.Sprintf("The data directory at %s/data contains database files\n"+
				"from a previous failed startup. This will cause 'invalid chain-id on InitChain' errors.\n\n"+
				"Run: monoctl node reset --home %s\n\n"+
				"This will clear all data and allow a fresh join.\n"+
				"Use --preserve-keys to keep your validator keys.",
				home, home),
		}
	}

	// Check 2: If node is already initialized, check for chain-id mismatch
	if IsNodeHomeInitialized(home) {
		existingChainID := GetExistingChainID(home)
		if existingChainID != "" && existingChainID != expectedChainID {
			return &PreflightError{
				Type:    "chain_id_mismatch",
				Message: fmt.Sprintf("Chain ID mismatch: existing genesis has %q, but joining %q", existingChainID, expectedChainID),
				Details: fmt.Sprintf("The node at %s was previously configured for chain %q.\n"+
					"To fix this, run: monoctl node reset --home %s\n"+
					"This will clear all data and allow a fresh join.",
					home, existingChainID, home),
			}
		}
	}

	return nil
}

// FindMonodBinary locates the monod binary.
// Search order: provided path, ~/bin/monod, /usr/local/bin/monod, PATH
func FindMonodBinary(providedPath string) (string, error) {
	if providedPath != "" {
		if _, err := os.Stat(providedPath); err == nil {
			return providedPath, nil
		}
		return "", fmt.Errorf("monod binary not found at: %s", providedPath)
	}

	// Check ~/bin/monod
	if home, err := os.UserHomeDir(); err == nil {
		localPath := filepath.Join(home, "bin", "monod")
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}

	// Check /usr/local/bin/monod
	if _, err := os.Stat("/usr/local/bin/monod"); err == nil {
		return "/usr/local/bin/monod", nil
	}

	// Check PATH
	path, err := exec.LookPath("monod")
	if err == nil {
		return path, nil
	}

	return "", fmt.Errorf("monod binary not found. Install it with: monoctl monod install")
}

// GenerateMoniker creates a moniker for the node.
// Uses hostname if available, otherwise generates a random suffix.
func GenerateMoniker() string {
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		// Clean hostname for use as moniker
		hostname = strings.ToLower(hostname)
		hostname = strings.ReplaceAll(hostname, ".", "-")
		if len(hostname) > 20 {
			hostname = hostname[:20]
		}
		return hostname
	}
	return "mono-node"
}

// InitializeNodeHome runs monod init to set up the node home directory.
func InitializeNodeHome(monodPath, home, moniker, chainID string, dryRun bool) (string, error) {
	if dryRun {
		return "", nil
	}

	// Run monod init
	cmd := exec.Command(monodPath, "init", moniker, "--chain-id", chainID, "--home", home)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("monod init failed: %w\nOutput: %s", err, string(output))
	}

	// Extract node_id from output (JSON format)
	// Output is like: {"moniker":"...", "chain_id":"...", "node_id":"..."}
	outputStr := string(output)
	nodeID := ""
	if idx := strings.Index(outputStr, `"node_id":"`); idx >= 0 {
		start := idx + len(`"node_id":"`)
		end := strings.Index(outputStr[start:], `"`)
		if end > 0 {
			nodeID = outputStr[start : start+end]
		}
	}

	return nodeID, nil
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

	// Step 3: Preflight checks (detect stale data, chain-id mismatch)
	logger.Info("running preflight checks")
	result.Steps = append(result.Steps, JoinStep{Name: "Preflight checks", Status: "pending"})

	if preflightErr := PreflightCheck(opts.Home, chainID); preflightErr != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = preflightErr.Message
		return result, fmt.Errorf("preflight check failed: %s\n\n%s", preflightErr.Message, preflightErr.Details)
	}
	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = "no issues detected"

	// Step 4: Initialize node home if not already initialized
	if !IsNodeHomeInitialized(opts.Home) {
		logger.Info("initializing node home", "home", opts.Home)
		result.Steps = append(result.Steps, JoinStep{Name: "Initialize node", Status: "pending"})

		// Find monod binary
		monodPath, err := FindMonodBinary(opts.MonodPath)
		if err != nil {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = err.Error()
			return result, fmt.Errorf("cannot initialize node: %w", err)
		}

		// Generate moniker if not provided
		moniker := opts.Moniker
		if moniker == "" {
			moniker = GenerateMoniker()
		}

		// Run monod init
		nodeID, err := InitializeNodeHome(monodPath, opts.Home, moniker, chainID, opts.DryRun)
		if err != nil {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = err.Error()
			return result, fmt.Errorf("failed to initialize node: %w", err)
		}

		result.NodeID = nodeID
		result.Initialized = true
		if opts.DryRun {
			result.Steps[len(result.Steps)-1].Status = "success"
			result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("(dry-run) moniker=%s", moniker)
		} else {
			result.Steps[len(result.Steps)-1].Status = "success"
			result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("moniker=%s, node_id=%s", moniker, nodeID)
		}
	} else {
		logger.Info("node already initialized, skipping init", "home", opts.Home)
		result.Steps = append(result.Steps, JoinStep{
			Name:    "Initialize node",
			Status:  "skipped",
			Message: "already initialized",
		})
	}

	// Step 5: Download peers to get genesis SHA256
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

	// Step 6: Verify SHA256 (use opts.GenesisSHA if provided, otherwise use from peers.json)
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

	// Step 7: Write genesis file
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

	// Step 8: Clear addrbook if in bootstrap mode
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

	// Step 9: Apply config
	logger.Info("applying config")
	result.Steps = append(result.Steps, JoinStep{Name: "Apply config", Status: "pending"})

	var patch *ConfigPatch
	if pexEnabled {
		patch = GenerateConfigPatch(seeds, persistentPeers)
	} else {
		patch = GenerateBootstrapConfigPatch(persistentPeers)
	}

	// Apply config patch directly to config.toml
	configPath := filepath.Join(opts.Home, "config", "config.toml")
	if err := ApplyConfigPatch(configPath, patch, opts.DryRun); err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		return result, fmt.Errorf("failed to apply config: %w", err)
	}

	// Build config patch content for display/logging
	var pexLine string
	if patch.PEX != nil {
		pexLine = fmt.Sprintf("pex = %v", *patch.PEX)
	}
	result.ConfigPatch = fmt.Sprintf("seeds=%q, persistent_peers=%q, %s", patch.Seeds, patch.PersistentPeers, pexLine)

	if opts.DryRun {
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = "(dry-run)"
	} else {
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = "config.toml updated"
	}

	// Step 10: Set chain-id in client.toml (CRITICAL for monod start)
	// Without this, monod start fails with "invalid chain-id on InitChain; expected: , got: <chain-id>"
	// because the SDK reads chain-id from client.toml, not genesis.json, for ABCI validation.
	logger.Info("setting chain-id in client.toml", "chain_id", chainID)
	result.Steps = append(result.Steps, JoinStep{Name: "Set client chain-id", Status: "pending"})

	if err := SetClientChainID(opts.Home, chainID, opts.DryRun); err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		return result, fmt.Errorf("failed to set chain-id in client.toml: %w", err)
	}

	if opts.DryRun {
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = "(dry-run)"
	} else {
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("chain-id = %q", chainID)
	}

	// Step 11: Set evm-chain-id in app.toml (CRITICAL for EVM determinism)
	// Without the correct evm-chain-id, nodes compute different state for EVM transactions,
	// causing AppHash mismatches and consensus failures.
	if network.EVMChainID != 0 {
		logger.Info("setting evm-chain-id in app.toml", "evm_chain_id", network.EVMChainID)
		result.Steps = append(result.Steps, JoinStep{Name: "Set EVM chain-id", Status: "pending"})

		if err := SetEVMChainID(opts.Home, network.EVMChainID, opts.DryRun); err != nil {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = err.Error()
			return result, fmt.Errorf("failed to set evm-chain-id in app.toml: %w", err)
		}

		if opts.DryRun {
			result.Steps[len(result.Steps)-1].Status = "success"
			result.Steps[len(result.Steps)-1].Message = "(dry-run)"
		} else {
			result.Steps[len(result.Steps)-1].Status = "success"
			result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("evm-chain-id = %d", network.EVMChainID)
		}
	}

	// Step 12: Detect and set external_address (CRITICAL for validators)
	// Without external_address, other validators cannot connect to receive block proposals,
	// causing the node to sign but never propose blocks.
	logger.Info("detecting public IP for external_address")
	result.Steps = append(result.Steps, JoinStep{Name: "Set external address", Status: "pending"})

	publicIP := net.DetectPublicIP()
	if publicIP == "" {
		result.Steps[len(result.Steps)-1].Status = "skipped"
		result.Steps[len(result.Steps)-1].Message = "could not detect public IP - set external_address manually if running as validator"
		logger.Warn("could not detect public IP, external_address not set")
	} else {
		externalAddr := fmt.Sprintf("tcp://%s:26656", publicIP)
		if err := SetExternalAddress(opts.Home, externalAddr, opts.DryRun); err != nil {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = err.Error()
			// Non-fatal: log warning and continue
			logger.Warn("failed to set external_address", "error", err)
		} else {
			if opts.DryRun {
				result.Steps[len(result.Steps)-1].Status = "success"
				result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("(dry-run) %s", externalAddr)
			} else {
				result.Steps[len(result.Steps)-1].Status = "success"
				result.Steps[len(result.Steps)-1].Message = externalAddr
			}
		}
	}

	result.Success = true
	return result, nil
}

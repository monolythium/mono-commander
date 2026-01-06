package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RepairResult describes what was repaired
type RepairResult struct {
	Field    string
	OldValue string
	NewValue string
	File     string
	Success  bool
	Error    string
}

// Repair fixes configuration drift by applying canonical config
func Repair(home string, config *DriftConfig, dryRun bool) ([]RepairResult, error) {
	var results []RepairResult
	configDir := filepath.Join(home, "config")

	// Repair client.toml chain-id
	clientPath := filepath.Join(configDir, "client.toml")
	oldChainID, _ := GetConfigValue(clientPath, "", "chain-id")
	oldChainID = strings.Trim(oldChainID, "\"")
	if err := SetClientChainID(home, config.CosmosChainID, dryRun); err != nil {
		results = append(results, RepairResult{
			Field:    "chain-id",
			OldValue: oldChainID,
			NewValue: config.CosmosChainID,
			File:     "client.toml",
			Success:  false,
			Error:    err.Error(),
		})
	} else {
		results = append(results, RepairResult{
			Field:    "chain-id",
			OldValue: oldChainID,
			NewValue: config.CosmosChainID,
			File:     "client.toml",
			Success:  true,
		})
	}

	// Repair app.toml evm-chain-id
	appPath := filepath.Join(configDir, "app.toml")
	oldEVMChainID, _ := GetConfigValue(appPath, "evm", "evm-chain-id")
	if err := SetEVMChainID(home, config.EVMChainID, dryRun); err != nil {
		results = append(results, RepairResult{
			Field:    "evm-chain-id",
			OldValue: oldEVMChainID,
			NewValue: fmt.Sprintf("%d", config.EVMChainID),
			File:     "app.toml",
			Success:  false,
			Error:    err.Error(),
		})
	} else {
		results = append(results, RepairResult{
			Field:    "evm-chain-id",
			OldValue: oldEVMChainID,
			NewValue: fmt.Sprintf("%d", config.EVMChainID),
			File:     "app.toml",
			Success:  true,
		})
	}

	// Repair config.toml p2p settings
	configPath := filepath.Join(configDir, "config.toml")

	// Create seeds from config
	seeds := make([]Peer, 0, len(config.Seeds))
	for _, s := range config.Seeds {
		if p, err := ParsePeer(s); err == nil {
			seeds = append(seeds, p)
		}
	}

	// Create bootstrap peers from config
	bootstrapPeers := make([]Peer, 0, len(config.BootstrapPeers))
	for _, s := range config.BootstrapPeers {
		if p, err := ParsePeer(s); err == nil {
			bootstrapPeers = append(bootstrapPeers, p)
		}
	}

	patch := GenerateConfigPatch(seeds, bootstrapPeers)
	if err := ApplyConfigPatch(configPath, patch, dryRun); err != nil {
		results = append(results, RepairResult{
			Field:   "p2p",
			File:    "config.toml",
			Success: false,
			Error:   err.Error(),
		})
	} else {
		results = append(results, RepairResult{
			Field:   "p2p",
			File:    "config.toml",
			Success: true,
		})
	}

	return results, nil
}

// FormatRepairReport formats repair results for display
func FormatRepairReport(results []RepairResult, dryRun bool) string {
	var sb strings.Builder

	if dryRun {
		sb.WriteString("DRY RUN - No changes made\n\n")
	}

	sb.WriteString("Repair Results:\n")

	allSuccess := true
	for _, r := range results {
		status := "✓"
		if !r.Success {
			status = "✗"
			allSuccess = false
		}

		if r.OldValue != "" && r.NewValue != "" {
			sb.WriteString(fmt.Sprintf("  %s %s %s: '%s' -> '%s'\n",
				status, r.File, r.Field, r.OldValue, r.NewValue))
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s %s\n", status, r.File, r.Field))
		}

		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("    Error: %s\n", r.Error))
		}
	}

	if allSuccess && !dryRun {
		sb.WriteString("\nAll repairs successful. Run 'monoctl doctor' to verify.\n")
	}

	return sb.String()
}

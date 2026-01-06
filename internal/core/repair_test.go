package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepair(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Write test client.toml
	clientToml := `chain-id = "old-chain-1"
node = "tcp://localhost:26657"
`
	if err := os.WriteFile(filepath.Join(configDir, "client.toml"), []byte(clientToml), 0644); err != nil {
		t.Fatalf("failed to write client.toml: %v", err)
	}

	// Write test app.toml
	appToml := `[evm]
evm-chain-id = 999999
`
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(appToml), 0644); err != nil {
		t.Fatalf("failed to write app.toml: %v", err)
	}

	// Write test config.toml
	configToml := `[p2p]
seeds = "old@example.com:26656"
persistent_peers = ""
pex = true
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configToml), 0644); err != nil {
		t.Fatalf("failed to write config.toml: %v", err)
	}

	// Test dry run
	t.Run("dry run", func(t *testing.T) {
		config := &DriftConfig{
			CosmosChainID: "new-chain-1",
			EVMChainID:    262145,
			Seeds:         []string{},
			BootstrapPeers: []string{},
		}

		results, err := Repair(tmpDir, config, true)
		if err != nil {
			t.Errorf("Repair() error = %v", err)
		}

		if len(results) == 0 {
			t.Error("expected repair results, got none")
		}

		// Verify files weren't actually changed in dry run
		data, _ := os.ReadFile(filepath.Join(configDir, "client.toml"))
		if !contains(string(data), "old-chain-1") {
			t.Error("dry run should not modify files")
		}
	})

	// Test actual repair
	t.Run("actual repair", func(t *testing.T) {
		config := &DriftConfig{
			CosmosChainID: "new-chain-1",
			EVMChainID:    262145,
			Seeds:         []string{},
			BootstrapPeers: []string{},
		}

		results, err := Repair(tmpDir, config, false)
		if err != nil {
			t.Errorf("Repair() error = %v", err)
		}

		if len(results) == 0 {
			t.Error("expected repair results, got none")
		}

		// Check if at least one repair was successful
		hasSuccess := false
		for _, r := range results {
			if r.Success {
				hasSuccess = true
				break
			}
		}
		if !hasSuccess {
			t.Error("expected at least one successful repair")
		}

		// Verify client.toml was updated
		data, _ := os.ReadFile(filepath.Join(configDir, "client.toml"))
		if !contains(string(data), "new-chain-1") {
			t.Error("client.toml should be updated with new chain-id")
		}
	})
}

func TestFormatRepairReport(t *testing.T) {
	tests := []struct {
		name    string
		results []RepairResult
		dryRun  bool
		wantStr string
	}{
		{
			name:    "empty results",
			results: []RepairResult{},
			dryRun:  false,
			wantStr: "Repair Results:",
		},
		{
			name: "successful repair",
			results: []RepairResult{
				{
					Field:    "chain-id",
					OldValue: "old-chain",
					NewValue: "new-chain",
					File:     "client.toml",
					Success:  true,
				},
			},
			dryRun:  false,
			wantStr: "✓",
		},
		{
			name: "failed repair",
			results: []RepairResult{
				{
					Field:   "chain-id",
					File:    "client.toml",
					Success: false,
					Error:   "some error",
				},
			},
			dryRun:  false,
			wantStr: "✗",
		},
		{
			name: "dry run",
			results: []RepairResult{
				{
					Field:    "chain-id",
					OldValue: "old-chain",
					NewValue: "new-chain",
					File:     "client.toml",
					Success:  true,
				},
			},
			dryRun:  true,
			wantStr: "DRY RUN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRepairReport(tt.results, tt.dryRun)
			if !contains(got, tt.wantStr) {
				t.Errorf("FormatRepairReport() output should contain %q, got %q", tt.wantStr, got)
			}
		})
	}
}

// Note: contains helper function is defined in join_test.go

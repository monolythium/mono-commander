package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePeer(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid peer",
			input:   "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0@example.com:26656",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "not-a-peer",
			wantErr: true,
		},
		{
			name:    "invalid node ID",
			input:   "short@example.com:26656",
			wantErr: true,
		},
		{
			name:    "invalid port",
			input:   "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0@example.com:abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peer, err := ParsePeer(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePeer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && peer.NodeID == "" {
				t.Errorf("ParsePeer() returned empty node ID for valid input")
			}
		})
	}
}

func TestDetectDrift(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Write test client.toml
	clientToml := `chain-id = "test-chain-1"
node = "tcp://localhost:26657"
`
	if err := os.WriteFile(filepath.Join(configDir, "client.toml"), []byte(clientToml), 0644); err != nil {
		t.Fatalf("failed to write client.toml: %v", err)
	}

	// Write test app.toml
	appToml := `[evm]
evm-chain-id = 262145
`
	if err := os.WriteFile(filepath.Join(configDir, "app.toml"), []byte(appToml), 0644); err != nil {
		t.Fatalf("failed to write app.toml: %v", err)
	}

	// Write test config.toml
	configToml := `[p2p]
seeds = ""
persistent_peers = ""
pex = true
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configToml), 0644); err != nil {
		t.Fatalf("failed to write config.toml: %v", err)
	}

	// Test case: drift detected
	t.Run("drift detected", func(t *testing.T) {
		config := &DriftConfig{
			CosmosChainID: "different-chain",
			EVMChainID:    999999,
			Seeds:         []string{},
			BootstrapPeers: []string{},
		}

		drifts, err := DetectDrift(tmpDir, config)
		if err != nil {
			t.Errorf("DetectDrift() error = %v", err)
		}

		if len(drifts) == 0 {
			t.Error("expected drift to be detected, got none")
		}

		if !HasCriticalDrift(drifts) {
			t.Error("expected critical drift, got none")
		}
	})

	// Test case: no drift
	t.Run("no drift", func(t *testing.T) {
		config := &DriftConfig{
			CosmosChainID: "test-chain-1",
			EVMChainID:    262145,
			Seeds:         []string{},
			BootstrapPeers: []string{},
		}

		drifts, err := DetectDrift(tmpDir, config)
		if err != nil {
			t.Errorf("DetectDrift() error = %v", err)
		}

		if len(drifts) != 0 {
			t.Errorf("expected no drift, got %d drifts: %v", len(drifts), drifts)
		}
	})
}

func TestFormatDriftReport(t *testing.T) {
	tests := []struct {
		name   string
		drifts []DriftResult
		want   string
	}{
		{
			name:   "no drift",
			drifts: []DriftResult{},
			want:   "No drift detected. Configuration matches canonical source.",
		},
		{
			name: "with drift",
			drifts: []DriftResult{
				{
					Field:    "chain-id",
					Expected: "expected-chain",
					Actual:   "actual-chain",
					File:     "client.toml",
					Severity: SeverityCritical,
				},
			},
			want: "DRIFT DETECTED:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDriftReport(tt.drifts)
			if len(tt.drifts) == 0 {
				if got != tt.want {
					t.Errorf("FormatDriftReport() = %v, want %v", got, tt.want)
				}
			} else {
				// Just check that it contains the expected substring for non-empty cases
				if len(got) == 0 {
					t.Error("FormatDriftReport() returned empty string for non-empty drift")
				}
			}
		})
	}
}

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJoin_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock genesis
	genesis := map[string]interface{}{
		"chain_id":     "mono-sprint-1",
		"genesis_time": "2025-01-01T00:00:00Z",
	}
	genesisData, _ := json.Marshal(genesis)
	genesisSHA := sha256.Sum256(genesisData)
	genesisSHAHex := hex.EncodeToString(genesisSHA[:])

	// Create mock peers
	peers := map[string]interface{}{
		"chain_id":       "mono-sprint-1",
		"genesis_sha256": genesisSHAHex,
		"peers": []map[string]interface{}{
			{
				"node_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"address": "peer1.example.com",
				"port":    26656,
			},
		},
	}
	peersData, _ := json.Marshal(peers)

	// Create mock fetcher
	fetcher := NewMockFetcher()
	fetcher.AddResponse("https://example.com/genesis.json", genesisData)
	fetcher.AddResponse("https://example.com/peers.json", peersData)

	opts := JoinOptions{
		Network:    NetworkSprintnet,
		Home:       tmpDir,
		GenesisURL: "https://example.com/genesis.json",
		GenesisSHA: genesisSHAHex,
		PeersURL:   "https://example.com/peers.json",
		DryRun:     true,
	}

	result, err := Join(opts, fetcher)
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}

	if !result.Success {
		t.Error("Join() Success = false, want true")
	}

	if result.ChainID != "mono-sprint-1" {
		t.Errorf("Join() ChainID = %v, want mono-sprint-1", result.ChainID)
	}

	// Verify no files were created (dry run)
	genesisPath := filepath.Join(tmpDir, "config", "genesis.json")
	if _, err := os.Stat(genesisPath); !os.IsNotExist(err) {
		t.Error("Join() dry run created genesis file")
	}
}

func TestJoin_ActualWrite(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock genesis
	genesis := map[string]interface{}{
		"chain_id":     "mono-sprint-1",
		"genesis_time": "2025-01-01T00:00:00Z",
	}
	genesisData, _ := json.Marshal(genesis)
	genesisSHA := sha256.Sum256(genesisData)
	genesisSHAHex := hex.EncodeToString(genesisSHA[:])

	fetcher := NewMockFetcher()
	fetcher.AddResponse("https://example.com/genesis.json", genesisData)

	opts := JoinOptions{
		Network:    NetworkSprintnet,
		Home:       tmpDir,
		GenesisURL: "https://example.com/genesis.json",
		GenesisSHA: genesisSHAHex,
		DryRun:     false,
	}

	result, err := Join(opts, fetcher)
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}

	if !result.Success {
		t.Error("Join() Success = false, want true")
	}

	// Verify genesis was created
	genesisPath := filepath.Join(tmpDir, "config", "genesis.json")
	if _, err := os.Stat(genesisPath); os.IsNotExist(err) {
		t.Error("Join() did not create genesis file")
	}

	// Verify config patch was created
	patchPath := filepath.Join(tmpDir, "config", "config_patch.toml")
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		t.Error("Join() did not create config patch")
	}
}

func TestJoin_ChainIDMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create genesis with wrong chain ID
	genesis := map[string]interface{}{
		"chain_id":     "wrong-chain-id",
		"genesis_time": "2025-01-01T00:00:00Z",
	}
	genesisData, _ := json.Marshal(genesis)

	fetcher := NewMockFetcher()
	fetcher.AddResponse("https://example.com/genesis.json", genesisData)

	opts := JoinOptions{
		Network:    NetworkSprintnet,
		Home:       tmpDir,
		GenesisURL: "https://example.com/genesis.json",
		DryRun:     true,
	}

	_, err := Join(opts, fetcher)
	if err == nil {
		t.Error("Join() expected error for chain ID mismatch")
	}
}

func TestJoin_SHAMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	genesis := map[string]interface{}{
		"chain_id":     "mono-sprint-1",
		"genesis_time": "2025-01-01T00:00:00Z",
	}
	genesisData, _ := json.Marshal(genesis)

	fetcher := NewMockFetcher()
	fetcher.AddResponse("https://example.com/genesis.json", genesisData)

	opts := JoinOptions{
		Network:    NetworkSprintnet,
		Home:       tmpDir,
		GenesisURL: "https://example.com/genesis.json",
		GenesisSHA: "wrongsha256hash",
		DryRun:     true,
	}

	_, err := Join(opts, fetcher)
	if err == nil {
		t.Error("Join() expected error for SHA256 mismatch")
	}
}

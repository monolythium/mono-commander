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
	// Skip if monod binary is not available (required for node init)
	if _, err := FindMonodBinary(""); err != nil {
		t.Skip("skipping test: monod binary not found")
	}

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
	// Skip if monod binary is not available (required for node init)
	if _, err := FindMonodBinary(""); err != nil {
		t.Skip("skipping test: monod binary not found")
	}

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

// TestJoin_SetsClientChainID verifies that Join() sets chain-id in client.toml
// This is critical: without this, monod start fails with
// "invalid chain-id on InitChain; expected: , got: <chain-id>"
func TestJoin_SetsClientChainID(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up config directory with client.toml (empty chain-id)
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create client.toml with empty chain-id (as monod init does by default in some cases)
	clientToml := `# Client config
chain-id = ""
keyring-backend = "os"
output = "text"
node = "tcp://localhost:26657"
broadcast-mode = "sync"
`
	if err := os.WriteFile(filepath.Join(configDir, "client.toml"), []byte(clientToml), 0644); err != nil {
		t.Fatalf("failed to write client.toml: %v", err)
	}

	// Also need config.toml for join to detect initialized state
	configToml := `[p2p]
seeds = ""
persistent_peers = ""
pex = true
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configToml), 0644); err != nil {
		t.Fatalf("failed to write config.toml: %v", err)
	}

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

	// Verify client.toml now has the correct chain-id
	clientPath := filepath.Join(configDir, "client.toml")
	data, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("failed to read client.toml: %v", err)
	}

	content := string(data)
	if !contains(content, `chain-id = "mono-sprint-1"`) {
		t.Errorf("client.toml should contain chain-id = \"mono-sprint-1\", got:\n%s", content)
	}
}

// TestJoin_FixesWrongClientChainID verifies that Join() corrects a wrong chain-id
func TestJoin_FixesWrongClientChainID(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up config directory with client.toml (wrong chain-id)
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create client.toml with wrong chain-id
	clientToml := `chain-id = "wrong-chain-id"
keyring-backend = "os"
`
	if err := os.WriteFile(filepath.Join(configDir, "client.toml"), []byte(clientToml), 0644); err != nil {
		t.Fatalf("failed to write client.toml: %v", err)
	}

	// Also need config.toml
	configToml := `[p2p]
seeds = ""
persistent_peers = ""
pex = true
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configToml), 0644); err != nil {
		t.Fatalf("failed to write config.toml: %v", err)
	}

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

	// Verify client.toml now has the correct chain-id (fixed from wrong value)
	clientPath := filepath.Join(configDir, "client.toml")
	data, err := os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("failed to read client.toml: %v", err)
	}

	content := string(data)
	if !contains(content, `chain-id = "mono-sprint-1"`) {
		t.Errorf("client.toml should contain chain-id = \"mono-sprint-1\", got:\n%s", content)
	}
	if contains(content, "wrong-chain-id") {
		t.Errorf("client.toml should not contain wrong-chain-id, got:\n%s", content)
	}
}

// TestJoin_PreflightDetectsDirtyState verifies preflight checks detect dirty data
func TestJoin_PreflightDetectsDirtyState(t *testing.T) {
	tmpDir := t.TempDir()

	// Create data directory with database files (simulating failed InitChain)
	dataDir := filepath.Join(tmpDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	// Create a state.db directory (LevelDB is a directory, not a file)
	stateDB := filepath.Join(dataDir, "state.db")
	if err := os.MkdirAll(stateDB, 0755); err != nil {
		t.Fatalf("failed to create state.db: %v", err)
	}

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

	_, err := Join(opts, fetcher)
	if err == nil {
		t.Error("Join() should fail when dirty state detected")
	}

	// Verify error message mentions dirty data
	if err != nil && !contains(err.Error(), "dirty") && !contains(err.Error(), "leftover") {
		t.Errorf("error should mention dirty/leftover state, got: %v", err)
	}
}

// TestHasStaleData verifies stale data detection
func TestHasStaleData(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(tmpDir string)
		expected bool
	}{
		{
			name:     "empty data dir",
			setup:    func(tmpDir string) {},
			expected: false,
		},
		{
			name: "only priv_validator_state.json",
			setup: func(tmpDir string) {
				dataDir := filepath.Join(tmpDir, "data")
				os.MkdirAll(dataDir, 0755)
				os.WriteFile(filepath.Join(dataDir, "priv_validator_state.json"), []byte("{}"), 0644)
			},
			expected: false,
		},
		{
			name: "has state.db",
			setup: func(tmpDir string) {
				dataDir := filepath.Join(tmpDir, "data")
				os.MkdirAll(filepath.Join(dataDir, "state.db"), 0755)
			},
			expected: true,
		},
		{
			name: "has application.db",
			setup: func(tmpDir string) {
				dataDir := filepath.Join(tmpDir, "data")
				os.MkdirAll(filepath.Join(dataDir, "application.db"), 0755)
			},
			expected: true,
		},
		{
			name: "has blockstore.db",
			setup: func(tmpDir string) {
				dataDir := filepath.Join(tmpDir, "data")
				os.MkdirAll(filepath.Join(dataDir, "blockstore.db"), 0755)
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(tmpDir)
			got := HasStaleData(tmpDir)
			if got != tt.expected {
				t.Errorf("HasStaleData() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

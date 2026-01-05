package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsConfigKey(t *testing.T) {
	tests := []struct {
		line     string
		key      string
		expected bool
	}{
		// Should match
		{`seeds = ""`, "seeds", true},
		{`seeds=""`, "seeds", true},
		{`seeds = "abc@host:1234"`, "seeds", true},
		{`  seeds = ""`, "seeds", true},
		{`persistent_peers = ""`, "persistent_peers", true},

		// Should NOT match - this was the bug that corrupted config
		{`experimental_max_gossip_connections_to_persistent_peers = 0`, "persistent_peers", false},
		{`experimental_max_gossip_connections_to_non_persistent_peers = 0`, "persistent_peers", false},

		// Should NOT match - comments
		{`# seeds = ""`, "seeds", false},
		{`  # seeds = ""`, "seeds", false},

		// Should NOT match - different keys
		{`other_seeds = ""`, "seeds", false},
		{`seedsomething = ""`, "seeds", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := isConfigKey(tt.line, tt.key)
			if result != tt.expected {
				t.Errorf("isConfigKey(%q, %q) = %v, want %v", tt.line, tt.key, result, tt.expected)
			}
		})
	}
}

func TestApplyConfigPatch_PreservesOtherKeys(t *testing.T) {
	// Create a temp config.toml with the problematic keys
	configContent := `# CometBFT Configuration

[p2p]
seeds = ""
persistent_peers = ""
experimental_max_gossip_connections_to_persistent_peers = 0
experimental_max_gossip_connections_to_non_persistent_peers = 0
max_num_inbound_peers = 40
max_num_outbound_peers = 10
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Apply patch
	patch := &ConfigPatch{
		Seeds:           "abc123@host1:26656,def456@host2:26656",
		PersistentPeers: "ghi789@host3:26656",
	}

	if err := ApplyConfigPatch(configPath, patch, false); err != nil {
		t.Fatalf("ApplyConfigPatch failed: %v", err)
	}

	// Read result
	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	resultStr := string(result)

	// Verify seeds was updated
	if !strings.Contains(resultStr, `seeds = "abc123@host1:26656,def456@host2:26656"`) {
		t.Errorf("seeds not updated correctly")
	}

	// Verify persistent_peers was updated
	if !strings.Contains(resultStr, `persistent_peers = "ghi789@host3:26656"`) {
		t.Errorf("persistent_peers not updated correctly")
	}

	// CRITICAL: Verify experimental keys were NOT corrupted
	if !strings.Contains(resultStr, `experimental_max_gossip_connections_to_persistent_peers = 0`) {
		t.Errorf("experimental_max_gossip_connections_to_persistent_peers was corrupted!")
	}
	if !strings.Contains(resultStr, `experimental_max_gossip_connections_to_non_persistent_peers = 0`) {
		t.Errorf("experimental_max_gossip_connections_to_non_persistent_peers was corrupted!")
	}

	// Verify other keys are preserved
	if !strings.Contains(resultStr, `max_num_inbound_peers = 40`) {
		t.Errorf("max_num_inbound_peers was lost")
	}
}

func TestApplyConfigPatch_OnlyModifiesP2PSection(t *testing.T) {
	// Config with multiple sections
	configContent := `[rpc]
seeds = "should_not_change"

[p2p]
seeds = ""
persistent_peers = ""

[mempool]
persistent_peers = "should_not_change"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	patch := &ConfigPatch{
		Seeds:           "new_seed@host:26656",
		PersistentPeers: "new_peer@host:26656",
	}

	if err := ApplyConfigPatch(configPath, patch, false); err != nil {
		t.Fatalf("ApplyConfigPatch failed: %v", err)
	}

	result, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read result: %v", err)
	}

	resultStr := string(result)

	// Verify only [p2p] section was modified
	if !strings.Contains(resultStr, `[rpc]
seeds = "should_not_change"`) {
		t.Errorf("[rpc] section was incorrectly modified")
	}

	if !strings.Contains(resultStr, `[mempool]
persistent_peers = "should_not_change"`) {
		t.Errorf("[mempool] section was incorrectly modified")
	}

	// Verify [p2p] section was updated
	if !strings.Contains(resultStr, `seeds = "new_seed@host:26656"`) {
		t.Errorf("[p2p] seeds not updated")
	}
}

func TestValidateConfigTOML(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		expectErr bool
	}{
		{
			name: "valid config",
			content: `[p2p]
seeds = ""
`,
			expectErr: false,
		},
		{
			name: "invalid TOML",
			content: `[p2p
seeds = ""
`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.toml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			err := ValidateConfigTOML(configPath)
			if tt.expectErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

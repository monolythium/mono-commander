package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// ConfigPatch represents changes to apply to config.toml.
type ConfigPatch struct {
	Seeds           string
	PersistentPeers string
	PEX             *bool  // nil = don't change, true/false = set value
}

// CometConfig represents the structure of config.toml for safe TOML editing.
// Only the fields we need to modify are defined; others are preserved via raw parsing.
type CometConfig struct {
	P2P P2PConfig `toml:"p2p"`
}

// P2PConfig represents the [p2p] section of config.toml.
type P2PConfig struct {
	Seeds           string `toml:"seeds"`
	PersistentPeers string `toml:"persistent_peers"`
	PEX             bool   `toml:"pex"`
}

// GenerateConfigPatch generates a config patch for the given network and peers.
// Seeds and persistent_peers MUST be in node_id@host:port format.
// The registry is the source of truth for all peer entries.
func GenerateConfigPatch(seeds []Peer, persistentPeers []Peer) *ConfigPatch {
	return &ConfigPatch{
		Seeds:           PeersToString(seeds),
		PersistentPeers: PeersToString(persistentPeers),
		PEX:             nil, // Don't change pex by default
	}
}

// GenerateBootstrapConfigPatch generates a config patch for bootstrap mode.
// This sets pex=false and uses bootstrap_peers as persistent_peers.
func GenerateBootstrapConfigPatch(bootstrapPeers []Peer) *ConfigPatch {
	pexFalse := false
	return &ConfigPatch{
		Seeds:           "",                           // No seeds in bootstrap mode
		PersistentPeers: PeersToString(bootstrapPeers),
		PEX:             &pexFalse,
	}
}

// WriteConfigPatch writes a config patch reference file (for documentation/review).
// Note: monoctl join now applies config directly; this function is kept for reference.
func WriteConfigPatch(home string, patch *ConfigPatch, dryRun bool) (string, string, error) {
	configDir := filepath.Join(home, "config")
	patchPath := filepath.Join(configDir, "config_patch.toml")

	var pexLine string
	if patch.PEX != nil {
		pexLine = fmt.Sprintf("pex = %v\n", *patch.PEX)
	}

	content := fmt.Sprintf(`# Mono Commander Config Patch
# These values have been applied to config.toml [p2p] section

[p2p]
seeds = "%s"
persistent_peers = "%s"
%s`, patch.Seeds, patch.PersistentPeers, pexLine)

	if dryRun {
		return patchPath, content, nil
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(patchPath, []byte(content), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write config patch: %w", err)
	}

	return patchPath, content, nil
}

// ApplyConfigPatch safely modifies config.toml using TOML-aware parsing.
// This preserves all existing config values and only updates the specified fields.
func ApplyConfigPatch(configPath string, patch *ConfigPatch, dryRun bool) error {
	if dryRun {
		return nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config.toml: %w", err)
	}

	// Use line-by-line parsing to preserve comments and formatting
	// This is safer than full TOML unmarshaling which loses comments
	lines := strings.Split(string(data), "\n")
	var result []string
	inP2PSection := false
	seedsReplaced := false
	peersReplaced := false
	pexReplaced := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track which section we're in
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inP2PSection = trimmed == "[p2p]"
		}

		// Replace seeds, persistent_peers, and pex in [p2p] section only
		if inP2PSection {
			if isConfigKey(trimmed, "seeds") {
				// Preserve leading whitespace
				leadingWS := getLeadingWhitespace(line)
				result = append(result, fmt.Sprintf(`%sseeds = "%s"`, leadingWS, patch.Seeds))
				seedsReplaced = true
				continue
			}
			if isConfigKey(trimmed, "persistent_peers") {
				leadingWS := getLeadingWhitespace(line)
				result = append(result, fmt.Sprintf(`%spersistent_peers = "%s"`, leadingWS, patch.PersistentPeers))
				peersReplaced = true
				continue
			}
			if patch.PEX != nil && isConfigKey(trimmed, "pex") {
				leadingWS := getLeadingWhitespace(line)
				result = append(result, fmt.Sprintf(`%spex = %v`, leadingWS, *patch.PEX))
				pexReplaced = true
				continue
			}
		}

		result = append(result, line)
	}

	// If we didn't find the keys to replace, we might need to add them
	// This handles malformed configs but shouldn't happen normally
	if !seedsReplaced {
		return fmt.Errorf("could not find 'seeds' key in [p2p] section of config.toml")
	}
	if !peersReplaced {
		return fmt.Errorf("could not find 'persistent_peers' key in [p2p] section of config.toml")
	}
	if patch.PEX != nil && !pexReplaced {
		return fmt.Errorf("could not find 'pex' key in [p2p] section of config.toml")
	}

	// Write back
	output := strings.Join(result, "\n")
	if err := os.WriteFile(configPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write config.toml: %w", err)
	}

	return nil
}

// isConfigKey checks if a line is a TOML key assignment for the given key name.
// This carefully avoids matching keys like "experimental_max_gossip_connections_to_persistent_peers"
// when looking for "persistent_peers".
func isConfigKey(line, key string) bool {
	// Remove leading/trailing whitespace
	trimmed := strings.TrimSpace(line)

	// Skip comments
	if strings.HasPrefix(trimmed, "#") {
		return false
	}

	// Check for exact key match: must be "key =" or "key="
	// The key must be at the start of the line (after trimming)
	if !strings.HasPrefix(trimmed, key) {
		return false
	}

	// Get what comes after the key name
	rest := trimmed[len(key):]

	// Must be followed by whitespace and/or equals sign
	rest = strings.TrimLeft(rest, " \t")
	return strings.HasPrefix(rest, "=")
}

// getLeadingWhitespace returns the leading whitespace of a line.
func getLeadingWhitespace(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

// ValidateConfigTOML validates that a config.toml file is valid TOML.
func ValidateConfigTOML(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Try to parse as TOML
	var parsed map[string]interface{}
	decoder := toml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&parsed); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}

	return nil
}

// GetConfigValue reads a specific value from config.toml using TOML parsing.
func GetConfigValue(configPath, section, key string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}

	var parsed map[string]interface{}
	decoder := toml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&parsed); err != nil {
		return "", fmt.Errorf("invalid TOML: %w", err)
	}

	sectionData, ok := parsed[section].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("section [%s] not found", section)
	}

	value, ok := sectionData[key]
	if !ok {
		return "", fmt.Errorf("key '%s' not found in section [%s]", key, section)
	}

	return fmt.Sprintf("%v", value), nil
}

// SetClientChainID sets the chain-id in client.toml.
// This is REQUIRED for monod start to work - without it, InitChain fails with
// "invalid chain-id on InitChain; expected: , got: <actual-chain-id>".
// The chain-id in client.toml is what monod uses to validate the genesis chain-id
// during the ABCI handshake.
func SetClientChainID(home string, chainID string, dryRun bool) error {
	if dryRun {
		return nil
	}

	clientPath := filepath.Join(home, "config", "client.toml")

	// Check if file exists
	if _, err := os.Stat(clientPath); os.IsNotExist(err) {
		return fmt.Errorf("client.toml not found at %s - run 'monod init' first", clientPath)
	}

	// Read existing config
	data, err := os.ReadFile(clientPath)
	if err != nil {
		return fmt.Errorf("failed to read client.toml: %w", err)
	}

	// Use line-by-line parsing to preserve comments and formatting
	lines := strings.Split(string(data), "\n")
	var result []string
	chainIDReplaced := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Replace chain-id line
		if isConfigKey(trimmed, "chain-id") {
			leadingWS := getLeadingWhitespace(line)
			result = append(result, fmt.Sprintf(`%schain-id = "%s"`, leadingWS, chainID))
			chainIDReplaced = true
			continue
		}

		result = append(result, line)
	}

	if !chainIDReplaced {
		return fmt.Errorf("could not find 'chain-id' key in client.toml")
	}

	// Write back
	output := strings.Join(result, "\n")
	if err := os.WriteFile(clientPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write client.toml: %w", err)
	}

	return nil
}

// ClearAddrbook removes the addrbook.json file to ensure fresh peer discovery.
// This is useful when switching sync strategies to avoid poisoned peers.
func ClearAddrbook(home string, dryRun bool) error {
	addrbookPath := filepath.Join(home, "config", "addrbook.json")

	// Check if file exists
	if _, err := os.Stat(addrbookPath); os.IsNotExist(err) {
		return nil // Nothing to clear
	}

	if dryRun {
		return nil
	}

	if err := os.Remove(addrbookPath); err != nil {
		return fmt.Errorf("failed to remove addrbook.json: %w", err)
	}

	return nil
}

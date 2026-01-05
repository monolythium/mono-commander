package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NodeRole represents the deployment role of a node.
type NodeRole string

const (
	// RoleFullNode is a normal node with pruning enabled and seed_mode=false.
	RoleFullNode NodeRole = "full_node"
	// RoleArchiveNode is an archive node with pruning disabled and seed_mode=false.
	RoleArchiveNode NodeRole = "archive_node"
	// RoleSeedNode is an official-grade seed requiring full archive and seed_mode=true.
	RoleSeedNode NodeRole = "seed_node"
)

// String returns the string representation of the role.
func (r NodeRole) String() string {
	return string(r)
}

// ParseNodeRole parses a string into a NodeRole.
func ParseNodeRole(s string) (NodeRole, error) {
	switch strings.ToLower(s) {
	case "full_node", "fullnode", "full":
		return RoleFullNode, nil
	case "archive_node", "archivenode", "archive":
		return RoleArchiveNode, nil
	case "seed_node", "seednode", "seed":
		return RoleSeedNode, nil
	default:
		return "", fmt.Errorf("invalid node role: %s (valid: full_node, archive_node, seed_node)", s)
	}
}

// AllNodeRoles returns all valid node roles.
func AllNodeRoles() []NodeRole {
	return []NodeRole{RoleFullNode, RoleArchiveNode, RoleSeedNode}
}

// RoleDescription returns a description for each role.
func RoleDescription(role NodeRole) string {
	switch role {
	case RoleFullNode:
		return "Normal node with pruning enabled, seed_mode=false"
	case RoleArchiveNode:
		return "Archive node with pruning disabled, seed_mode=false"
	case RoleSeedNode:
		return "Official seed: requires full archive (pruning=nothing), seed_mode=true, earliest_block_height=1"
	default:
		return "Unknown role"
	}
}

// RoleConfig contains the configuration settings required for each role.
type RoleConfig struct {
	SeedMode            bool
	Pruning             string
	PruningKeepRecent   string
	PruningInterval     string
	MinRetainBlocks     int
	IndexerEnabled      bool
	EarliestHeightCheck bool // Whether earliest_block_height=1 is required
}

// GetRoleConfig returns the configuration requirements for a given role.
func GetRoleConfig(role NodeRole) RoleConfig {
	switch role {
	case RoleFullNode:
		return RoleConfig{
			SeedMode:            false,
			Pruning:             "custom",
			PruningKeepRecent:   "100",
			PruningInterval:     "10",
			MinRetainBlocks:     0,
			IndexerEnabled:      true,
			EarliestHeightCheck: false,
		}
	case RoleArchiveNode:
		return RoleConfig{
			SeedMode:            false,
			Pruning:             "nothing",
			PruningKeepRecent:   "0",
			PruningInterval:     "0",
			MinRetainBlocks:     0,
			IndexerEnabled:      true,
			EarliestHeightCheck: false,
		}
	case RoleSeedNode:
		return RoleConfig{
			SeedMode:            true,
			Pruning:             "nothing",
			PruningKeepRecent:   "0",
			PruningInterval:     "0",
			MinRetainBlocks:     0,
			IndexerEnabled:      false, // Seeds don't need indexer
			EarliestHeightCheck: true,  // Must have earliest_block_height=1
		}
	default:
		// Default to full node config
		return GetRoleConfig(RoleFullNode)
	}
}

// RoleValidationResult contains the result of validating a node's role configuration.
type RoleValidationResult struct {
	Valid       bool
	Role        NodeRole
	Issues      []RoleIssue
	Suggestions []string
}

// RoleIssue represents a configuration issue related to node roles.
type RoleIssue struct {
	Severity string // "CRITICAL", "WARNING", "INFO"
	Field    string
	Expected string
	Actual   string
	Message  string
}

// ValidateRoleConfig validates that a node's configuration matches its declared role.
func ValidateRoleConfig(home string, declaredRole NodeRole) (*RoleValidationResult, error) {
	result := &RoleValidationResult{
		Valid:       true,
		Role:        declaredRole,
		Issues:      make([]RoleIssue, 0),
		Suggestions: make([]string, 0),
	}

	expected := GetRoleConfig(declaredRole)

	// Read config.toml
	configPath := filepath.Join(home, "config", "config.toml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config.toml: %w", err)
	}

	// Read app.toml
	appPath := filepath.Join(home, "config", "app.toml")
	appData, err := os.ReadFile(appPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read app.toml: %w", err)
	}

	// Check seed_mode in config.toml
	actualSeedMode := getConfigBool(string(configData), "p2p", "seed_mode")
	if actualSeedMode != expected.SeedMode {
		severity := "CRITICAL"
		if declaredRole == RoleSeedNode && !actualSeedMode {
			severity = "CRITICAL"
		} else if declaredRole != RoleSeedNode && actualSeedMode {
			severity = "WARNING"
		}
		result.Issues = append(result.Issues, RoleIssue{
			Severity: severity,
			Field:    "seed_mode",
			Expected: fmt.Sprintf("%t", expected.SeedMode),
			Actual:   fmt.Sprintf("%t", actualSeedMode),
			Message:  fmt.Sprintf("seed_mode should be %t for %s role", expected.SeedMode, declaredRole),
		})
		result.Valid = false
	}

	// Check pruning in app.toml
	actualPruning := getConfigString(string(appData), "", "pruning")
	if actualPruning != expected.Pruning {
		severity := "WARNING"
		if declaredRole == RoleSeedNode && actualPruning != "nothing" {
			severity = "CRITICAL"
		}
		result.Issues = append(result.Issues, RoleIssue{
			Severity: severity,
			Field:    "pruning",
			Expected: expected.Pruning,
			Actual:   actualPruning,
			Message:  fmt.Sprintf("pruning should be '%s' for %s role", expected.Pruning, declaredRole),
		})
		result.Valid = false
	}

	// CRITICAL: seed_mode=true MUST have pruning=nothing
	if actualSeedMode && actualPruning != "nothing" {
		result.Issues = append(result.Issues, RoleIssue{
			Severity: "CRITICAL",
			Field:    "pruning+seed_mode",
			Expected: "pruning=nothing when seed_mode=true",
			Actual:   fmt.Sprintf("pruning=%s with seed_mode=true", actualPruning),
			Message:  "UNSAFE: seed_mode=true with pruning enabled. Seeds must be full archive to serve genesis blocksync.",
		})
		result.Valid = false
	}

	// Generate suggestions
	if len(result.Issues) > 0 {
		for _, issue := range result.Issues {
			switch {
			case issue.Field == "seed_mode" && issue.Severity == "CRITICAL":
				result.Suggestions = append(result.Suggestions,
					fmt.Sprintf("Run: monoctl node configure --role %s --home %s", declaredRole, home))
			case issue.Field == "pruning" && issue.Severity == "CRITICAL":
				result.Suggestions = append(result.Suggestions,
					"Seed nodes require pruning=nothing. Change pruning or disable seed_mode.")
			case issue.Field == "pruning+seed_mode":
				result.Suggestions = append(result.Suggestions,
					"CRITICAL: Either set pruning=nothing or disable seed_mode. Seeds must serve full history.")
			}
		}
	}

	return result, nil
}

// DetectCurrentRole attempts to detect the current role based on configuration.
func DetectCurrentRole(home string) (NodeRole, error) {
	// Read config.toml
	configPath := filepath.Join(home, "config", "config.toml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config.toml: %w", err)
	}

	// Read app.toml
	appPath := filepath.Join(home, "config", "app.toml")
	appData, err := os.ReadFile(appPath)
	if err != nil {
		return "", fmt.Errorf("failed to read app.toml: %w", err)
	}

	seedMode := getConfigBool(string(configData), "p2p", "seed_mode")
	pruning := getConfigString(string(appData), "", "pruning")

	if seedMode {
		return RoleSeedNode, nil
	}

	if pruning == "nothing" {
		return RoleArchiveNode, nil
	}

	return RoleFullNode, nil
}

// ApplyRoleConfig applies the configuration for a given role to config.toml and app.toml.
func ApplyRoleConfig(home string, role NodeRole, dryRun bool) error {
	expected := GetRoleConfig(role)

	// Apply seed_mode to config.toml
	configPath := filepath.Join(home, "config", "config.toml")
	if err := applyConfigValue(configPath, "p2p", "seed_mode", fmt.Sprintf("%t", expected.SeedMode), dryRun); err != nil {
		return fmt.Errorf("failed to set seed_mode: %w", err)
	}

	// Apply pruning settings to app.toml
	appPath := filepath.Join(home, "config", "app.toml")
	if err := applyConfigValue(appPath, "", "pruning", fmt.Sprintf("\"%s\"", expected.Pruning), dryRun); err != nil {
		return fmt.Errorf("failed to set pruning: %w", err)
	}
	if err := applyConfigValue(appPath, "", "pruning-keep-recent", fmt.Sprintf("\"%s\"", expected.PruningKeepRecent), dryRun); err != nil {
		return fmt.Errorf("failed to set pruning-keep-recent: %w", err)
	}
	if err := applyConfigValue(appPath, "", "pruning-interval", fmt.Sprintf("\"%s\"", expected.PruningInterval), dryRun); err != nil {
		return fmt.Errorf("failed to set pruning-interval: %w", err)
	}

	return nil
}

// getConfigBool reads a boolean value from a TOML config string.
func getConfigBool(data, section, key string) bool {
	value := getConfigString(data, section, key)
	return value == "true"
}

// getConfigString reads a string value from a TOML config string.
func getConfigString(data, section, key string) string {
	lines := strings.Split(data, "\n")
	inSection := section == ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track section
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection := strings.Trim(trimmed, "[]")
			inSection = (section == "" || currentSection == section)
			continue
		}

		if !inSection {
			continue
		}

		// Check for key
		if strings.HasPrefix(trimmed, key) {
			rest := strings.TrimPrefix(trimmed, key)
			rest = strings.TrimLeft(rest, " \t")
			if strings.HasPrefix(rest, "=") {
				value := strings.TrimPrefix(rest, "=")
				value = strings.TrimSpace(value)
				// Remove quotes if present
				value = strings.Trim(value, "\"")
				return value
			}
		}
	}

	return ""
}

// applyConfigValue modifies a value in a TOML config file.
func applyConfigValue(path, section, key, value string, dryRun bool) error {
	if dryRun {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	inSection := section == ""
	keyFound := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track section
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection := strings.Trim(trimmed, "[]")
			inSection = (section == "" || currentSection == section)
		}

		// Check for key to replace
		if inSection && strings.HasPrefix(trimmed, key) {
			rest := strings.TrimPrefix(trimmed, key)
			rest = strings.TrimLeft(rest, " \t")
			if strings.HasPrefix(rest, "=") {
				// Preserve leading whitespace
				leadingWS := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				result = append(result, fmt.Sprintf("%s%s = %s", leadingWS, key, value))
				keyFound = true
				continue
			}
		}

		result = append(result, line)
	}

	if !keyFound {
		return fmt.Errorf("key '%s' not found in section [%s]", key, section)
	}

	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644)
}

// IsSeedModeAllowed checks if seed_mode=true is allowed given the current pruning setting.
func IsSeedModeAllowed(home string) (bool, string) {
	appPath := filepath.Join(home, "config", "app.toml")
	appData, err := os.ReadFile(appPath)
	if err != nil {
		return false, "Cannot read app.toml"
	}

	pruning := getConfigString(string(appData), "", "pruning")
	if pruning != "nothing" {
		return false, fmt.Sprintf("seed_mode requires pruning=nothing, but pruning=%s. Seeds must be full archive nodes.", pruning)
	}

	return true, ""
}

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ConfigPatch represents changes to apply to config.toml.
type ConfigPatch struct {
	Seeds           string
	PersistentPeers string
}

// configTOMLTemplate is a minimal template for patching config.toml.
// In production, we'd use sed/toml parsing; for now we generate a patch file.
const configPatchTemplate = `# Mono Commander Config Patch
# Apply these values to your config.toml [p2p] section

[p2p]
{{- if .Seeds }}
seeds = "{{ .Seeds }}"
{{- end }}
{{- if .PersistentPeers }}
persistent_peers = "{{ .PersistentPeers }}"
{{- end }}
`

// GenerateConfigPatch generates a config patch for the given network and peers.
func GenerateConfigPatch(network Network, peers []Peer) *ConfigPatch {
	return &ConfigPatch{
		Seeds:           network.SeedString(26656),
		PersistentPeers: PeersToString(peers),
	}
}

// WriteConfigPatch writes the config patch to a file.
func WriteConfigPatch(home string, patch *ConfigPatch, dryRun bool) (string, string, error) {
	configDir := filepath.Join(home, "config")
	patchPath := filepath.Join(configDir, "config_patch.toml")

	tmpl, err := template.New("config").Parse(configPatchTemplate)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, patch); err != nil {
		return "", "", fmt.Errorf("failed to execute template: %w", err)
	}

	content := buf.String()

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

// ApplyConfigPatch modifies an existing config.toml with the patch values.
// This is a simple implementation that appends instructions.
func ApplyConfigPatch(configPath string, patch *ConfigPatch, dryRun bool) error {
	if dryRun {
		return nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config.toml: %w", err)
	}

	content := string(data)

	// Simple replacement (in production, use a proper TOML parser)
	if patch.Seeds != "" {
		// Find and replace seeds line
		content = replaceConfigValue(content, "seeds", patch.Seeds)
	}

	if patch.PersistentPeers != "" {
		content = replaceConfigValue(content, "persistent_peers", patch.PersistentPeers)
	}

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config.toml: %w", err)
	}

	return nil
}

// replaceConfigValue replaces a TOML key value in the content.
func replaceConfigValue(content, key, value string) string {
	lines := strings.Split(content, "\n")
	found := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = fmt.Sprintf(`%s = "%s"`, key, value)
			found = true
			break
		}
	}

	if !found {
		// Append to end of [p2p] section or file
		lines = append(lines, fmt.Sprintf(`%s = "%s"`, key, value))
	}

	return strings.Join(lines, "\n")
}

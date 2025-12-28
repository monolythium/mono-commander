// Package mesh provides Mesh/Rosetta API sidecar management for mono-commander.
package mesh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/monolythium/mono-commander/internal/core"
)

// SystemdConfig holds configuration for generating the mesh sidecar systemd unit.
type SystemdConfig struct {
	Network     string
	User        string
	BinaryPath  string
	ConfigPath  string
	Description string
	After       string
	Restart     string
	RestartSec  int
}

// DefaultSystemdConfig returns a default systemd configuration for the mesh sidecar.
func DefaultSystemdConfig(network, user, home string, netName core.NetworkName) *SystemdConfig {
	return &SystemdConfig{
		Network:     network,
		User:        user,
		BinaryPath:  BinaryInstallPath(false),
		ConfigPath:  ConfigPath(home, netName),
		Description: fmt.Sprintf("Mesh/Rosetta API Sidecar (%s)", network),
		After:       "network-online.target",
		Restart:     "on-failure",
		RestartSec:  10,
	}
}

// systemdMeshTemplate is the template for generating the mesh sidecar systemd unit.
const systemdMeshTemplate = `[Unit]
Description={{ .Description }}
After={{ .After }}
Wants=network-online.target

[Service]
User={{ .User }}
Group={{ .User }}
Type=simple
ExecStart={{ .BinaryPath }} --config {{ .ConfigPath }}
Restart={{ .Restart }}
RestartSec={{ .RestartSec }}
LimitNOFILE=65535

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths={{ .ConfigDir }}

[Install]
WantedBy=multi-user.target
`

// UnitName returns the systemd unit name for a network.
func UnitName(network string) string {
	return fmt.Sprintf("mono-mesh@%s.service", strings.ToLower(network))
}

// UnitPath returns the full path to the systemd unit file.
func UnitPath(network string) string {
	return filepath.Join("/etc/systemd/system", UnitName(network))
}

// GenerateSystemdUnit generates the systemd unit file content.
func GenerateSystemdUnit(cfg *SystemdConfig) (string, error) {
	// Create a version with ConfigDir for ReadWritePaths
	type templateData struct {
		*SystemdConfig
		ConfigDir string
	}

	data := templateData{
		SystemdConfig: cfg,
		ConfigDir:     filepath.Dir(cfg.ConfigPath),
	}

	tmpl, err := template.New("mesh-systemd").Parse(systemdMeshTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// WriteSystemdUnit writes the systemd unit file.
func WriteSystemdUnit(cfg *SystemdConfig, dryRun bool) (string, string, error) {
	content, err := GenerateSystemdUnit(cfg)
	if err != nil {
		return "", "", err
	}

	unitPath := UnitPath(cfg.Network)

	if dryRun {
		return unitPath, content, nil
	}

	// Check if we can write to systemd directory
	if _, err := os.Stat("/etc/systemd/system"); os.IsNotExist(err) {
		return "", "", fmt.Errorf("systemd not available: /etc/systemd/system does not exist")
	}

	if err := os.WriteFile(unitPath, []byte(content), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write unit file (do you need sudo?): %w", err)
	}

	return unitPath, content, nil
}

// EnableResult contains the result of enabling the service.
type EnableResult struct {
	UnitPath   string
	UnitName   string
	ConfigPath string
	Enabled    bool
	Started    bool
	Error      error
}

// EnableService enables and starts the systemd service.
func EnableService(network string, dryRun bool) (*EnableResult, error) {
	unitName := UnitName(network)

	result := &EnableResult{
		UnitPath: UnitPath(network),
		UnitName: unitName,
	}

	if dryRun {
		return result, nil
	}

	// Reload systemd daemon
	if err := runSystemctl("daemon-reload"); err != nil {
		result.Error = fmt.Errorf("failed to reload daemon: %w", err)
		return result, result.Error
	}

	// Enable the service
	if err := runSystemctl("enable", unitName); err != nil {
		result.Error = fmt.Errorf("failed to enable service: %w", err)
		return result, result.Error
	}
	result.Enabled = true

	// Start the service
	if err := runSystemctl("start", unitName); err != nil {
		result.Error = fmt.Errorf("failed to start service: %w", err)
		return result, result.Error
	}
	result.Started = true

	return result, nil
}

// DisableResult contains the result of disabling the service.
type DisableResult struct {
	UnitName string
	Stopped  bool
	Disabled bool
	Error    error
}

// DisableService stops and disables the systemd service.
func DisableService(network string, dryRun bool) (*DisableResult, error) {
	unitName := UnitName(network)

	result := &DisableResult{
		UnitName: unitName,
	}

	if dryRun {
		return result, nil
	}

	// Stop the service
	if err := runSystemctl("stop", unitName); err != nil {
		// Don't fail if service is not running
		if !strings.Contains(err.Error(), "not loaded") {
			result.Error = fmt.Errorf("failed to stop service: %w", err)
			return result, result.Error
		}
	}
	result.Stopped = true

	// Disable the service
	if err := runSystemctl("disable", unitName); err != nil {
		// Don't fail if service was not enabled
		if !strings.Contains(err.Error(), "not loaded") {
			result.Error = fmt.Errorf("failed to disable service: %w", err)
			return result, result.Error
		}
	}
	result.Disabled = true

	return result, nil
}

// ServiceStatus represents the status of the systemd service.
type ServiceStatus struct {
	Active      bool
	ActiveState string
	SubState    string
	MainPID     int
	Error       error
}

// GetServiceStatus gets the status of the systemd service.
func GetServiceStatus(network string) *ServiceStatus {
	unitName := UnitName(network)
	status := &ServiceStatus{}

	// Get active state
	out, err := exec.Command("systemctl", "is-active", unitName).Output()
	if err == nil {
		state := strings.TrimSpace(string(out))
		status.ActiveState = state
		status.Active = state == "active"
	} else {
		status.ActiveState = "inactive"
	}

	// Get sub-state
	out, err = exec.Command("systemctl", "show", unitName, "--property=SubState").Output()
	if err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(out)), "=", 2)
		if len(parts) == 2 {
			status.SubState = parts[1]
		}
	}

	// Get main PID
	out, err = exec.Command("systemctl", "show", unitName, "--property=MainPID").Output()
	if err == nil {
		parts := strings.SplitN(strings.TrimSpace(string(out)), "=", 2)
		if len(parts) == 2 {
			fmt.Sscanf(parts[1], "%d", &status.MainPID)
		}
	}

	return status
}

// runSystemctl runs a systemctl command.
func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}
	return nil
}

// SystemdInstructions returns manual instructions for enabling the service.
func SystemdInstructions(unitPath, unitName string) string {
	return fmt.Sprintf(`
Systemd unit file written to: %s

To enable and start the service:
  sudo systemctl daemon-reload
  sudo systemctl enable %s
  sudo systemctl start %s

To check status:
  sudo systemctl status %s

To view logs:
  sudo journalctl -u %s -f
`, unitPath, unitName, unitName, unitName, unitName)
}

// IsSystemdAvailable checks if systemd is available on the system.
func IsSystemdAvailable() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

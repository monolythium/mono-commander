// Package os provides OS-level operations for mono-commander.
package os

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// SystemdConfig holds configuration for generating systemd unit files.
type SystemdConfig struct {
	Network     string
	User        string
	Home        string
	BinaryPath  string
	Description string
	After       string
	Restart     string
	RestartSec  int
	// Cosmovisor settings (optional)
	UseCosmovisor bool
	CosmovisorBin string
}

// DefaultSystemdConfig returns a default systemd configuration.
func DefaultSystemdConfig(network, user, home string) *SystemdConfig {
	return &SystemdConfig{
		Network:       network,
		User:          user,
		Home:          home,
		BinaryPath:    "/usr/local/bin/monod",
		Description:   fmt.Sprintf("Monolythium Node (%s)", network),
		After:         "network-online.target",
		Restart:       "on-failure",
		RestartSec:    10,
		UseCosmovisor: false,
		CosmovisorBin: "/usr/local/bin/cosmovisor",
	}
}

// systemdTemplate is the template for generating systemd unit files.
const systemdTemplate = `[Unit]
Description={{ .Description }}
After={{ .After }}
Wants=network-online.target

[Service]
User={{ .User }}
Group={{ .User }}
Type=simple
{{- if .UseCosmovisor }}
Environment="DAEMON_NAME=monod"
Environment="DAEMON_HOME={{ .Home }}"
Environment="DAEMON_ALLOW_DOWNLOAD_BINARIES=false"
Environment="DAEMON_RESTART_AFTER_UPGRADE=true"
Environment="DAEMON_POLL_INTERVAL=300ms"
Environment="UNSAFE_SKIP_BACKUP=true"
ExecStart={{ .CosmovisorBin }} run start --home {{ .Home }}
{{- else }}
ExecStart={{ .BinaryPath }} start --home {{ .Home }}
{{- end }}
Restart={{ .Restart }}
RestartSec={{ .RestartSec }}
LimitNOFILE=65535

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths={{ .Home }}

[Install]
WantedBy=multi-user.target
`

// GenerateSystemdUnit generates a systemd unit file.
func GenerateSystemdUnit(cfg *SystemdConfig) (string, error) {
	tmpl, err := template.New("systemd").Parse(systemdTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf string
	builder := &stringWriter{buf: &buf}
	if err := tmpl.Execute(builder, cfg); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf, nil
}

// WriteSystemdUnit writes a systemd unit file.
func WriteSystemdUnit(cfg *SystemdConfig, dryRun bool) (string, string, error) {
	content, err := GenerateSystemdUnit(cfg)
	if err != nil {
		return "", "", err
	}

	unitName := fmt.Sprintf("monod-%s.service", cfg.Network)
	unitPath := filepath.Join("/etc/systemd/system", unitName)

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

// SystemdInstructions returns instructions for enabling the service.
func SystemdInstructions(unitPath string) string {
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
`, unitPath, filepath.Base(unitPath), filepath.Base(unitPath), filepath.Base(unitPath), filepath.Base(unitPath))
}

// stringWriter is a simple io.Writer that appends to a string.
type stringWriter struct {
	buf *string
}

func (w *stringWriter) Write(p []byte) (n int, err error) {
	*w.buf += string(p)
	return len(p), nil
}

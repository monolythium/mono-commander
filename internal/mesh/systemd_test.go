package mesh

import (
	"strings"
	"testing"

	"github.com/monolythium/mono-commander/internal/core"
)

func TestUnitName(t *testing.T) {
	tests := []struct {
		network string
		want    string
	}{
		{"Localnet", "mono-mesh@localnet.service"},
		{"Sprintnet", "mono-mesh@sprintnet.service"},
		{"Testnet", "mono-mesh@testnet.service"},
		{"Mainnet", "mono-mesh@mainnet.service"},
	}

	for _, tt := range tests {
		got := UnitName(tt.network)
		if got != tt.want {
			t.Errorf("UnitName(%s) = %s, want %s", tt.network, got, tt.want)
		}
	}
}

func TestUnitPath(t *testing.T) {
	path := UnitPath("Sprintnet")
	expected := "/etc/systemd/system/mono-mesh@sprintnet.service"

	if path != expected {
		t.Errorf("UnitPath() = %s, want %s", path, expected)
	}
}

func TestDefaultSystemdConfig(t *testing.T) {
	cfg := DefaultSystemdConfig("Sprintnet", "monod", "/home/monod", core.NetworkSprintnet)

	if cfg.Network != "Sprintnet" {
		t.Errorf("DefaultSystemdConfig() Network = %s, want Sprintnet", cfg.Network)
	}

	if cfg.User != "monod" {
		t.Errorf("DefaultSystemdConfig() User = %s, want monod", cfg.User)
	}

	if cfg.RestartSec != 10 {
		t.Errorf("DefaultSystemdConfig() RestartSec = %d, want 10", cfg.RestartSec)
	}

	if cfg.Restart != "on-failure" {
		t.Errorf("DefaultSystemdConfig() Restart = %s, want on-failure", cfg.Restart)
	}
}

func TestGenerateSystemdUnit(t *testing.T) {
	cfg := DefaultSystemdConfig("Sprintnet", "monod", "/home/monod", core.NetworkSprintnet)

	content, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit() error = %v", err)
	}

	// Verify required sections
	requiredParts := []string{
		"[Unit]",
		"[Service]",
		"[Install]",
		"Description=Mesh/Rosetta API Sidecar (Sprintnet)",
		"User=monod",
		"Restart=on-failure",
		"RestartSec=10",
		"WantedBy=multi-user.target",
	}

	for _, part := range requiredParts {
		if !strings.Contains(content, part) {
			t.Errorf("GenerateSystemdUnit() missing %q", part)
		}
	}
}

func TestGenerateSystemdUnit_SecurityHardening(t *testing.T) {
	cfg := DefaultSystemdConfig("Localnet", "testuser", "/home/testuser", core.NetworkLocalnet)

	content, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit() error = %v", err)
	}

	// Verify security settings are present
	securitySettings := []string{
		"NoNewPrivileges=true",
		"PrivateTmp=true",
		"ProtectSystem=strict",
		"ProtectHome=read-only",
	}

	for _, setting := range securitySettings {
		if !strings.Contains(content, setting) {
			t.Errorf("GenerateSystemdUnit() missing security setting %q", setting)
		}
	}
}

func TestWriteSystemdUnit_DryRun(t *testing.T) {
	cfg := DefaultSystemdConfig("Testnet", "monod", "/home/monod", core.NetworkTestnet)

	unitPath, content, err := WriteSystemdUnit(cfg, true)
	if err != nil {
		t.Fatalf("WriteSystemdUnit(dry-run) error = %v", err)
	}

	if unitPath == "" {
		t.Error("WriteSystemdUnit(dry-run) returned empty unitPath")
	}

	if content == "" {
		t.Error("WriteSystemdUnit(dry-run) returned empty content")
	}

	// Verify path format
	if !strings.Contains(unitPath, "mono-mesh@testnet.service") {
		t.Errorf("WriteSystemdUnit(dry-run) unitPath = %s, should contain mono-mesh@testnet.service", unitPath)
	}
}

func TestSystemdInstructions(t *testing.T) {
	unitPath := "/etc/systemd/system/mono-mesh@sprintnet.service"
	unitName := "mono-mesh@sprintnet.service"

	instructions := SystemdInstructions(unitPath, unitName)

	// Verify instructions contain key commands
	requiredParts := []string{
		"systemctl daemon-reload",
		"systemctl enable",
		"systemctl start",
		"systemctl status",
		"journalctl -u",
		unitPath,
		unitName,
	}

	for _, part := range requiredParts {
		if !strings.Contains(instructions, part) {
			t.Errorf("SystemdInstructions() missing %q", part)
		}
	}
}

func TestDisableService_DryRun(t *testing.T) {
	result, err := DisableService("Localnet", true)
	if err != nil {
		t.Fatalf("DisableService(dry-run) error = %v", err)
	}

	if result.UnitName == "" {
		t.Error("DisableService(dry-run) returned empty UnitName")
	}

	// Dry run should not set Stopped or Disabled
	if result.Stopped {
		t.Error("DisableService(dry-run) should not set Stopped")
	}

	if result.Disabled {
		t.Error("DisableService(dry-run) should not set Disabled")
	}
}

func TestEnableService_DryRun(t *testing.T) {
	result, err := EnableService("Localnet", true)
	if err != nil {
		t.Fatalf("EnableService(dry-run) error = %v", err)
	}

	if result.UnitName == "" {
		t.Error("EnableService(dry-run) returned empty UnitName")
	}

	// Dry run should not set Enabled or Started
	if result.Enabled {
		t.Error("EnableService(dry-run) should not set Enabled")
	}

	if result.Started {
		t.Error("EnableService(dry-run) should not set Started")
	}
}

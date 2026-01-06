package os

import (
	"strings"
	"testing"
)

func TestGenerateSystemdUnit(t *testing.T) {
	cfg := DefaultSystemdConfig("Sprintnet", "monod", "/home/monod/.monod")

	content, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit() error = %v", err)
	}

	// Verify required sections
	if !strings.Contains(content, "[Unit]") {
		t.Error("Missing [Unit] section")
	}
	if !strings.Contains(content, "[Service]") {
		t.Error("Missing [Service] section")
	}
	if !strings.Contains(content, "[Install]") {
		t.Error("Missing [Install] section")
	}

	// Verify key values
	if !strings.Contains(content, "User=monod") {
		t.Error("Missing User directive")
	}
	if !strings.Contains(content, "/home/monod/.monod") {
		t.Error("Missing home directory")
	}
	if !strings.Contains(content, "Monolythium Node (Sprintnet)") {
		t.Error("Missing description")
	}

	// Verify security hardening
	if !strings.Contains(content, "NoNewPrivileges=true") {
		t.Error("Missing security hardening")
	}

	// CRITICAL: Verify explicit --home flag in ExecStart
	// Without this, monod uses default ~/.monod which may differ from monoctl join target
	if !strings.Contains(content, "ExecStart=/usr/local/bin/monod start --home /home/monod/.monod") {
		t.Errorf("ExecStart missing explicit --home flag, got content:\n%s", content)
	}

	// Verify WorkingDirectory matches Home
	if !strings.Contains(content, "WorkingDirectory=/home/monod/.monod") {
		t.Errorf("WorkingDirectory should match Home, got content:\n%s", content)
	}
}

func TestGenerateSystemdUnit_Cosmovisor(t *testing.T) {
	cfg := DefaultSystemdConfig("Mainnet", "monod", "/home/monod/.monod")
	cfg.UseCosmovisor = true

	content, err := GenerateSystemdUnit(cfg)
	if err != nil {
		t.Fatalf("GenerateSystemdUnit() error = %v", err)
	}

	// Verify Cosmovisor settings
	if !strings.Contains(content, "DAEMON_NAME=monod") {
		t.Error("Missing DAEMON_NAME environment")
	}
	if !strings.Contains(content, "DAEMON_HOME=/home/monod/.monod") {
		t.Error("Missing DAEMON_HOME environment")
	}
	if !strings.Contains(content, "cosmovisor run start") {
		t.Error("Missing cosmovisor ExecStart")
	}
}

func TestWriteSystemdUnit_DryRun(t *testing.T) {
	cfg := DefaultSystemdConfig("Sprintnet", "monod", "/home/monod/.monod")

	path, content, err := WriteSystemdUnit(cfg, true)
	if err != nil {
		t.Fatalf("WriteSystemdUnit() dry run error = %v", err)
	}

	if path != "/etc/systemd/system/monod-Sprintnet.service" {
		t.Errorf("WriteSystemdUnit() path = %v, want /etc/systemd/system/monod-Sprintnet.service", path)
	}

	if content == "" {
		t.Error("WriteSystemdUnit() returned empty content")
	}
}

func TestSystemdInstructions(t *testing.T) {
	instructions := SystemdInstructions("/etc/systemd/system/monod-Sprintnet.service")

	if !strings.Contains(instructions, "systemctl daemon-reload") {
		t.Error("Missing daemon-reload instruction")
	}
	if !strings.Contains(instructions, "systemctl enable") {
		t.Error("Missing enable instruction")
	}
	if !strings.Contains(instructions, "systemctl start") {
		t.Error("Missing start instruction")
	}
	if !strings.Contains(instructions, "journalctl") {
		t.Error("Missing journalctl instruction")
	}
}

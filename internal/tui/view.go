package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Render tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// Render tab content
	switch m.activeTab {
	case TabDashboard:
		b.WriteString(m.renderDashboard())
	case TabHealth:
		b.WriteString(m.renderHealth())
	case TabLogs:
		b.WriteString(m.renderLogs())
	case TabUpdate:
		b.WriteString(m.renderUpdate())
	case TabInstall:
		b.WriteString(m.renderInstall())
	case TabHelp:
		b.WriteString(m.renderHelp())
	}

	// Render status line
	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(m.status))
	}

	// Render error if any
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}

	// Render help footer
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpFooter())

	return b.String()
}

func (m Model) renderTabBar() string {
	var tabs []string

	for _, t := range m.tabs {
		var style lipgloss.Style
		if t == m.activeTab {
			style = activeTabStyle
		} else {
			style = tabStyle
		}
		tabs = append(tabs, style.Render(t.String()))
	}

	return tabBarStyle.Width(m.width).Render(strings.Join(tabs, " "))
}

func (m Model) renderHelpFooter() string {
	var help string
	switch m.activeTab {
	case TabDashboard:
		help = "Tab/←→: switch tabs • n: change network • r: refresh • q: quit"
	case TabHealth:
		help = "Tab/←→: switch tabs • r: refresh • 1/2/3: section • q: quit"
	case TabLogs:
		help = "Tab/←→: switch tabs • c: configure • s: start/stop • f: toggle follow • q: quit"
	case TabUpdate:
		help = "Tab/←→: switch tabs • r: check updates • u: update commander • q: quit"
	case TabInstall:
		help = "Tab/←→: switch tabs • 1: deps • 2: monod • 3: join • 4: mesh • q: quit"
	case TabHelp:
		help = "Tab/←→: switch tabs • q: quit"
	}

	if m.subView != SubViewNone {
		help = "Esc: back • " + help
	}

	return helpStyle.Render(help)
}

// Dashboard rendering
func (m Model) renderDashboard() string {
	var b strings.Builder

	// Network selector subview
	if m.subView == SubViewNetworkSelect {
		return m.list.View()
	}

	// Header with network
	header := fmt.Sprintf("Dashboard - %s", m.selectedNetwork)
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString(" Loading...")
		return b.String()
	}

	if m.dashboardData == nil {
		b.WriteString("  Press 'r' to refresh")
		return b.String()
	}

	d := m.dashboardData

	// Installation status section
	b.WriteString(sectionStyle.Render(titleStyle.Render("Installation Status")))
	b.WriteString("\n")

	installTable := [][]string{
		{"monod binary", formatStatus(d.MonodInstalled, d.MonodVersion)},
		{"Node home (~/.monod)", formatBool(d.HomeExists)},
		{"genesis.json", formatBool(d.GenesisExists)},
	}
	b.WriteString(renderTable(installTable, 2))
	b.WriteString("\n")

	// Service status section
	b.WriteString(sectionStyle.Render(titleStyle.Render("Service Status")))
	b.WriteString("\n")

	serviceTable := [][]string{
		{"monod systemd", formatServiceStatus(d.ServiceStatus)},
		{"Mesh/Rosetta sidecar", formatServiceStatus(d.MeshStatus)},
	}
	b.WriteString(renderTable(serviceTable, 2))
	b.WriteString("\n")

	// Versions section
	b.WriteString(sectionStyle.Render(titleStyle.Render("Versions")))
	b.WriteString("\n")

	commanderVersion := Version
	if commanderVersion == "" || commanderVersion == "dev" {
		commanderVersion = "dev"
	}

	versionTable := [][]string{
		{"Commander", commanderVersion + " ✓"},
		{"monod", orNA(d.MonodVersion)},
	}
	b.WriteString(renderTable(versionTable, 2))
	b.WriteString("\n")

	// Node status section (if available)
	if d.NodeStatus != nil {
		b.WriteString(sectionStyle.Render(titleStyle.Render("Node Status")))
		b.WriteString("\n")

		syncStatus := "✓ synced"
		if d.NodeStatus.CatchingUp {
			syncStatus = "⟳ catching up"
		}

		nodeTable := [][]string{
			{"Chain ID", d.NodeStatus.ChainID},
			{"Latest Height", fmt.Sprintf("%d %s", d.NodeStatus.LatestHeight, syncStatus)},
			{"Peers", fmt.Sprintf("%d", d.NodeStatus.PeersCount)},
		}
		b.WriteString(renderTable(nodeTable, 2))
		b.WriteString("\n")
	}

	// Update status
	if d.CommanderUpdate != nil && d.CommanderUpdate.UpdateAvailable {
		b.WriteString("\n")
		b.WriteString(warningStyle.Render(fmt.Sprintf(
			"⚠ Commander update available: %s → %s (press 'u' in Update tab)",
			d.CommanderUpdate.CurrentVersion,
			d.CommanderUpdate.LatestVersion,
		)))
	}

	b.WriteString("\n")
	b.WriteString(statusStyle.Render(fmt.Sprintf("Last refresh: %s", d.LastRefresh.Format("15:04:05"))))

	return b.String()
}

// Health rendering
func (m Model) renderHealth() string {
	var b strings.Builder

	header := fmt.Sprintf("Health - %s", m.selectedNetwork)
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString(" Checking health...")
		return b.String()
	}

	if m.healthData == nil {
		b.WriteString("  Press 'r' to run health checks")
		return b.String()
	}

	h := m.healthData

	// System Requirements
	if h.SystemHealth != nil {
		b.WriteString(sectionStyle.Render(titleStyle.Render("[1] System Requirements")))
		b.WriteString("\n")

		sysTable := [][]string{
			{"OS / Arch", fmt.Sprintf("%s / %s", h.SystemHealth.OS, h.SystemHealth.Arch)},
			{"CPU Count", fmt.Sprintf("%d", h.SystemHealth.CPUCount)},
			{"RAM", formatBytes(h.SystemHealth.RAMTotal) + " total, " + formatBytes(h.SystemHealth.RAMFree) + " free"},
			{"Disk Free", formatBytes(h.SystemHealth.DiskFree)},
		}
		b.WriteString(renderTable(sysTable, 2))
		b.WriteString("\n")

		// Ports
		b.WriteString("  Ports:\n")
		for _, p := range h.SystemHealth.Ports {
			status := "✗ closed"
			if p.Listening {
				status = "✓ listening"
			}
			b.WriteString(fmt.Sprintf("    %-20s %d %s\n", p.Name, p.Port, status))
		}
		b.WriteString("\n")
	}

	// Node Health
	if h.NodeHealth != nil {
		b.WriteString(sectionStyle.Render(titleStyle.Render("[2] Node Health")))
		b.WriteString("\n")

		renderRPCStatus := func(name string, s *RPCStatus) string {
			if s == nil {
				return fmt.Sprintf("  %-15s [?] not checked", name)
			}
			status := "[PASS]"
			if s.Status == "FAIL" {
				status = "[FAIL]"
			}
			line := fmt.Sprintf("  %-15s %s %s", name, status, s.Endpoint)
			if s.Details != "" {
				line += "\n                    " + s.Details
			}
			if s.Error != "" {
				line += "\n                    Error: " + s.Error
			}
			return line
		}

		b.WriteString(renderRPCStatus("Comet RPC", h.NodeHealth.CometStatus))
		b.WriteString("\n")
		b.WriteString(renderRPCStatus("Cosmos REST", h.NodeHealth.CosmosStatus))
		b.WriteString("\n")
		b.WriteString(renderRPCStatus("EVM JSON-RPC", h.NodeHealth.EVMStatus))
		b.WriteString("\n")

		if h.NodeHealth.Height > 0 {
			syncStatus := "synced"
			if h.NodeHealth.CatchingUp {
				syncStatus = "catching up"
			}
			b.WriteString(fmt.Sprintf("\n  Height: %d (%s), Peers: %d\n", h.NodeHealth.Height, syncStatus, h.NodeHealth.Peers))
		}
		b.WriteString("\n")
	}

	// Multi-node health (if applicable)
	if len(h.MultiNodeHealth) > 0 {
		b.WriteString(sectionStyle.Render(titleStyle.Render("Multi-Node Status")))
		b.WriteString("\n")
		for _, n := range h.MultiNodeHealth {
			b.WriteString(fmt.Sprintf("  %s: height=%d catching_up=%v\n", n.NodeName, n.Height, n.CatchingUp))
		}
		b.WriteString("\n")
	}

	// Validator Health
	if h.ValidatorHealth != nil {
		b.WriteString(sectionStyle.Render(titleStyle.Render("[3] Validator Health")))
		b.WriteString("\n")

		if h.ValidatorHealth.NotConfigured {
			b.WriteString("  Validator not configured (no operator key detected)\n")
		} else {
			valTable := [][]string{
				{"Valoper Address", h.ValidatorHealth.ValoperAddr},
				{"Status", h.ValidatorHealth.Status},
				{"Jailed", fmt.Sprintf("%v", h.ValidatorHealth.Jailed)},
			}
			if h.ValidatorHealth.MissedBlocks > 0 {
				valTable = append(valTable, []string{"Missed Blocks", fmt.Sprintf("%d", h.ValidatorHealth.MissedBlocks)})
			}
			b.WriteString(renderTable(valTable, 2))
		}
	}

	b.WriteString("\n")
	b.WriteString(statusStyle.Render(fmt.Sprintf("Last refresh: %s", h.LastRefresh.Format("15:04:05"))))

	return b.String()
}

// Logs rendering
func (m Model) renderLogs() string {
	var b strings.Builder

	header := fmt.Sprintf("Logs - %s", m.selectedNetwork)
	b.WriteString(titleStyle.Render(header))
	b.WriteString("\n\n")

	// Config subview
	if m.subView == SubViewForm {
		return m.renderForm()
	}

	// Logs viewer
	if m.subView == SubViewLogsViewer && m.logsData.Streaming {
		b.WriteString(fmt.Sprintf("  Streaming: %s / %s", m.logsData.Network, m.logsData.Service))
		if m.logsData.Follow {
			b.WriteString(" (following)")
		}
		b.WriteString("\n\n")
		b.WriteString(m.viewport.View())
		return b.String()
	}

	// Config display
	l := m.logsData
	configTable := [][]string{
		{"Network", orValue(l.Network, string(m.selectedNetwork))},
		{"Service", orValue(l.Service, "monod")},
		{"Lines", fmt.Sprintf("%d", l.Lines)},
		{"Follow", fmt.Sprintf("%v", l.Follow)},
		{"Filter", orValue(l.Filter, "(none)")},
	}

	b.WriteString("  Current Configuration:\n")
	b.WriteString(renderTable(configTable, 4))
	b.WriteString("\n\n")
	b.WriteString("  Press 'c' to configure, 's' to start streaming\n")

	// Show recent logs if any
	if len(l.LogLines) > 0 {
		b.WriteString("\n  Recent logs:\n")
		start := len(l.LogLines) - 10
		if start < 0 {
			start = 0
		}
		for _, line := range l.LogLines[start:] {
			b.WriteString("    " + truncate(line, m.width-8) + "\n")
		}
	}

	return b.String()
}

// Update rendering
func (m Model) renderUpdate() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Update"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString(" Checking for updates...")
		return b.String()
	}

	if m.updateData == nil {
		b.WriteString("  Press 'r' to check for updates")
		return b.String()
	}

	u := m.updateData

	// Commander update
	b.WriteString(sectionStyle.Render(titleStyle.Render("Commander")))
	b.WriteString("\n")

	cmdStatus := "✓ Up to date"
	if u.CommanderUpdate {
		cmdStatus = warningStyle.Render("⚠ Update available")
	}

	cmdTable := [][]string{
		{"Current", u.CommanderCurrent},
		{"Latest", u.CommanderLatest},
		{"Status", cmdStatus},
	}
	b.WriteString(renderTable(cmdTable, 2))
	b.WriteString("\n")

	// monod update
	b.WriteString(sectionStyle.Render(titleStyle.Render("monod")))
	b.WriteString("\n")

	monodTable := [][]string{
		{"Current", orNA(u.MonodCurrent)},
		{"Latest", u.MonodLatest},
	}
	b.WriteString(renderTable(monodTable, 2))
	if u.MonodLatest == "check manually" {
		b.WriteString("  Note: Update source not configured\n")
	}
	b.WriteString("\n")

	// Sidecar update
	b.WriteString(sectionStyle.Render(titleStyle.Render("Mesh/Rosetta Sidecar")))
	b.WriteString("\n")

	sidecarTable := [][]string{
		{"Current", orNA(u.SidecarCurrent)},
		{"Latest", u.SidecarLatest},
	}
	b.WriteString(renderTable(sidecarTable, 2))
	if u.SidecarLatest == "check manually" {
		b.WriteString("  Note: Update source not configured\n")
	}

	b.WriteString("\n")
	b.WriteString(statusStyle.Render(fmt.Sprintf("Last check: %s", u.CheckedAt.Format("15:04:05"))))

	return b.String()
}

// Install rendering
func (m Model) renderInstall() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("(Re)Install"))
	b.WriteString("\n\n")

	// Form subview
	if m.subView == SubViewForm {
		return m.renderForm()
	}

	// Menu
	b.WriteString("  Select an option:\n\n")

	options := []struct {
		key   string
		title string
		desc  string
	}{
		{"1", "System Dependencies", "Check and install required dependencies (curl, jq, etc.)"},
		{"2", "Install monod", "Install the Monolythium node binary"},
		{"3", "Join Network", "Download genesis and configure peers for a network"},
		{"4", "Install Mesh/Rosetta", "Install the Rosetta API sidecar"},
	}

	for _, opt := range options {
		b.WriteString(fmt.Sprintf("  [%s] %s\n", opt.key, opt.title))
		b.WriteString(fmt.Sprintf("      %s\n\n", statusStyle.Render(opt.desc)))
	}

	// Show install status
	i := m.installData
	if i != nil {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render(titleStyle.Render("Current Status")))
		b.WriteString("\n")

		statusTable := [][]string{
			{"Dependencies", formatBool(i.DepsInstalled)},
			{"monod", orNA(i.MonodVersion)},
			{"Network Join", orNA(i.JoinStatus)},
			{"Mesh/Rosetta", formatBool(i.MeshInstalled)},
		}
		b.WriteString(renderTable(statusTable, 2))
	}

	return b.String()
}

// Help rendering
func (m Model) renderHelp() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Help"))
	b.WriteString("\n\n")

	// What is Commander
	b.WriteString(sectionStyle.Render(titleStyle.Render("What is Mono Commander?")))
	b.WriteString("\n")
	b.WriteString(`  Mono Commander is a TUI-first tool for installing and operating
  Monolythium nodes across Localnet, Sprintnet, Testnet, and Mainnet.
`)
	b.WriteString("\n")

	// Safety
	b.WriteString(sectionStyle.Render(titleStyle.Render("Safety Constraints")))
	b.WriteString("\n")
	b.WriteString(`  • No secrets stored: Commander never stores mnemonics, private keys, or tokens
  • No key generation: Key management is handled by the node binary
  • No rollback: Emergency recovery is HALT → PATCH → UPGRADE → RESTART
  • Dry-run recommended: Always preview changes before applying
`)
	b.WriteString("\n")

	// Shortcuts
	b.WriteString(sectionStyle.Render(titleStyle.Render("Keyboard Shortcuts")))
	b.WriteString("\n")
	shortcuts := [][]string{
		{"Tab / Shift+Tab", "Switch between tabs"},
		{"← / →", "Navigate tabs"},
		{"Enter", "Select / Confirm"},
		{"Esc", "Go back / Cancel"},
		{"r", "Refresh current view"},
		{"q", "Quit"},
	}
	b.WriteString(renderTable(shortcuts, 2))
	b.WriteString("\n")

	// Tab-specific shortcuts
	b.WriteString("  Dashboard: n = change network\n")
	b.WriteString("  Health: 1/2/3 = section details\n")
	b.WriteString("  Logs: c = configure, s = start/stop, f = follow\n")
	b.WriteString("  Update: u = update commander\n")
	b.WriteString("  Install: 1/2/3/4 = wizard steps\n")
	b.WriteString("\n")

	// Troubleshooting
	b.WriteString(sectionStyle.Render(titleStyle.Render("Troubleshooting")))
	b.WriteString("\n")
	b.WriteString(`  RPC unreachable:
    • Check if the node is running: systemctl status monod
    • Verify the node is listening on expected ports

  Wrong chain-id / EVM chain-id:
    • Ensure you're connected to the correct network
    • Check genesis.json matches the expected network

  Systemd not present:
    • Systemd is required for service management on Linux
    • On macOS, use launchd or run manually

  Ports in use:
    • Stop conflicting services: lsof -i :26657
    • Use different ports in config.toml
`)
	b.WriteString("\n")

	// Links
	b.WriteString(sectionStyle.Render(titleStyle.Render("Documentation")))
	b.WriteString("\n")
	b.WriteString("  • GitHub: https://github.com/monolythium/mono-commander\n")
	b.WriteString("  • Docs: https://docs.monolythium.com\n")

	return b.String()
}

// Form rendering
func (m Model) renderForm() string {
	var b strings.Builder

	// Title based on context
	title := "Form"
	switch m.subView {
	case SubViewInstallMonod:
		title = "Install monod"
	case SubViewInstallJoin:
		title = "Join Network"
	case SubViewInstallMesh:
		title = "Install Mesh/Rosetta"
	case SubViewLogsConfig:
		title = "Configure Logs"
	}

	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Render form fields
	for i, field := range m.formFields {
		label := field.Label
		if field.Required {
			label += " *"
		}

		style := blurredStyle
		if i == m.formIndex {
			style = focusedStyle
		}

		b.WriteString("  ")
		b.WriteString(style.Render(label))
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(field.Input.View())
		b.WriteString("\n\n")
	}

	b.WriteString(statusStyle.Render("  Tab/↓: next • Shift+Tab/↑: prev • Enter: submit • Esc: cancel"))

	return b.String()
}

// Helper functions

func formatStatus(installed bool, version string) string {
	if !installed {
		return "✗ not installed"
	}
	if version != "" {
		return "✓ " + version
	}
	return "✓ installed"
}

func formatBool(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func formatServiceStatus(status string) string {
	switch status {
	case "active":
		return successStyle.Render("✓ active")
	case "inactive":
		return "○ inactive"
	case "failed":
		return errorStyle.Render("✗ failed")
	case "not found", "not installed":
		return "- not installed"
	default:
		return status
	}
}

func formatBytes(bytes uint64) string {
	if bytes == 0 {
		return "N/A"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func orValue(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func renderTable(rows [][]string, indent int) string {
	if len(rows) == 0 {
		return ""
	}

	// Find max width for first column
	maxWidth := 0
	for _, row := range rows {
		if len(row) > 0 && len(row[0]) > maxWidth {
			maxWidth = len(row[0])
		}
	}

	var b strings.Builder
	for _, row := range rows {
		b.WriteString(strings.Repeat(" ", indent))
		if len(row) >= 2 {
			b.WriteString(fmt.Sprintf("%-*s  %s", maxWidth, row[0], row[1]))
		} else if len(row) == 1 {
			b.WriteString(row[0])
		}
		b.WriteString("\n")
	}
	return b.String()
}

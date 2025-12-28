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

	// Render tab bar with positions for mouse detection
	tabBar, _ := RenderTabBar(m.tabs, m.activeTab, m.width)
	b.WriteString(tabBar)
	b.WriteString("\n")

	// Calculate content height (total - tab bar - footer)
	contentHeight := m.height - 6

	// Render tab content
	var content string
	switch m.activeTab {
	case TabDashboard:
		content = m.renderDashboard()
	case TabHealth:
		content = m.renderHealth()
	case TabLogs:
		content = m.renderLogs()
	case TabUpdate:
		content = m.renderUpdate()
	case TabInstall:
		content = m.renderInstall()
	case TabHelp:
		content = m.renderHelp()
	}

	// Wrap content to fit height
	contentStyle := lipgloss.NewStyle().
		Height(contentHeight).
		MaxHeight(contentHeight)
	b.WriteString(contentStyle.Render(content))

	// Render status line
	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(TextMuted.MarginLeft(2).Render(m.status))
	}

	// Render error if any
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(TextDanger.MarginLeft(2).Render("Error: " + m.err.Error()))
	}

	// Render help footer
	b.WriteString("\n")
	b.WriteString(m.renderHelpFooter())

	return b.String()
}

func (m Model) renderHelpFooter() string {
	var hints []string

	// Common navigation hints
	hints = append(hints, KeyHint("Tab", "switch"))
	hints = append(hints, KeyHint("←→", "tabs"))

	// Tab-specific hints
	switch m.activeTab {
	case TabDashboard:
		hints = append(hints, KeyHint("n", "network"))
		hints = append(hints, KeyHint("r", "refresh"))
	case TabHealth:
		hints = append(hints, KeyHint("r", "refresh"))
		hints = append(hints, KeyHint("1/2/3", "section"))
	case TabLogs:
		hints = append(hints, KeyHint("c", "config"))
		hints = append(hints, KeyHint("s", "stream"))
		hints = append(hints, KeyHint("f", "follow"))
	case TabUpdate:
		hints = append(hints, KeyHint("r", "check"))
		hints = append(hints, KeyHint("u", "update"))
	case TabInstall:
		hints = append(hints, KeyHint("1-4", "steps"))
	case TabHelp:
		hints = append(hints, KeyHint("↑↓", "scroll"))
	}

	hints = append(hints, KeyHint("q", "quit"))

	if m.subView != SubViewNone {
		hints = append([]string{KeyHint("Esc", "back")}, hints...)
	}

	return ActionBar(hints, m.width)
}

// Dashboard rendering with cards layout
func (m Model) renderDashboard() string {
	var b strings.Builder

	// Network selector subview
	if m.subView == SubViewNetworkSelect {
		return m.list.View()
	}

	// Page header
	lastRefresh := ""
	if m.dashboardData != nil && !m.dashboardData.LastRefresh.IsZero() {
		lastRefresh = "Last refresh: " + m.dashboardData.LastRefresh.Format("15:04:05")
	}
	b.WriteString(PageHeader(
		fmt.Sprintf("Dashboard · %s", m.selectedNetwork),
		lastRefresh,
	))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString(" Loading...")
		return b.String()
	}

	if m.dashboardData == nil {
		b.WriteString(TextMuted.Render("  Press 'r' to refresh"))
		return b.String()
	}

	d := m.dashboardData
	cardWidth := (m.width - 8) / 2
	if cardWidth < 35 {
		cardWidth = m.width - 6
	}

	// Row 1: Network + Install cards side by side
	networkCard := m.renderNetworkCard(cardWidth)
	installCard := m.renderInstallStatusCard(cardWidth)

	if cardWidth < m.width-6 {
		// Side by side
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, networkCard, "  ", installCard))
	} else {
		// Stacked
		b.WriteString(networkCard)
		b.WriteString("\n")
		b.WriteString(installCard)
	}
	b.WriteString("\n\n")

	// Row 2: Node Status card (if node is running)
	if d.NodeStatus != nil {
		nodeCard := m.renderNodeStatusCard(m.width - 6)
		b.WriteString(nodeCard)
		b.WriteString("\n")

		// Check for network mismatch
		expectedChainID := getExpectedChainID(m.selectedNetwork)
		if expectedChainID != "" && d.NodeStatus.ChainID != expectedChainID {
			b.WriteString("\n")
			b.WriteString(NetworkMismatchWarning(string(m.selectedNetwork), d.NodeStatus.ChainID, m.width-6))
		}
	}

	// Commander update notification
	if d.CommanderUpdate != nil && d.CommanderUpdate.UpdateAvailable {
		b.WriteString("\n")
		updateMsg := fmt.Sprintf(
			"Commander update available: %s → %s",
			d.CommanderUpdate.CurrentVersion,
			d.CommanderUpdate.LatestVersion,
		)
		b.WriteString(WarningBox("Update Available", updateMsg+"\n\nPress 'u' in Update tab to update.", m.width-6))
	}

	// Actions strip
	b.WriteString("\n")
	actions := KeyHints(
		KeyHint("n", "change network"),
		KeyHint("r", "refresh"),
	)
	b.WriteString(TextMuted.Render("  " + actions))

	return b.String()
}

func (m Model) renderNetworkCard(width int) string {
	network := m.selectedNetwork
	var chainID, evmID string

	// Get network info
	for _, n := range m.networks {
		if n.Name == network {
			chainID = n.ChainID
			evmID = fmt.Sprintf("%d (0x%x)", n.EVMChainID, n.EVMChainID)
			break
		}
	}

	rows := [][]string{
		{"Network", string(network)},
		{"Chain ID", chainID},
		{"EVM Chain ID", evmID},
	}

	body := Table(rows, 0)
	return Card("Network", body, width)
}

func (m Model) renderInstallStatusCard(width int) string {
	d := m.dashboardData

	rows := []StatusRow{
		{Label: "monod binary", Status: boolToStatus(d.MonodInstalled), Value: orEmpty(d.MonodVersion, "OK")},
		{Label: "Node home", Status: boolToStatus(d.HomeExists)},
		{Label: "genesis.json", Status: boolToStatus(d.GenesisExists)},
		{Label: "Systemd service", Status: serviceToStatus(d.ServiceStatus)},
		{Label: "Mesh/Rosetta", Status: serviceToStatus(d.MeshStatus)},
	}

	body := StatusTable(rows, 0)
	return Card("Installation Status", body, width)
}

func (m Model) renderNodeStatusCard(width int) string {
	d := m.dashboardData
	ns := d.NodeStatus

	// Sync status with color
	var syncBadge string
	if ns.CatchingUp {
		syncBadge = Badge(BadgeWarn, "SYNCING")
	} else {
		syncBadge = Badge(BadgeOK, "SYNCED")
	}

	rows := [][]string{
		{"Chain ID", ns.ChainID},
		{"Latest Height", fmt.Sprintf("%d  %s", ns.LatestHeight, syncBadge)},
		{"Peers", fmt.Sprintf("%d", ns.PeersCount)},
	}

	body := Table(rows, 0)
	return Card("Node Status", body, width)
}

// Health rendering with semantic colors
func (m Model) renderHealth() string {
	var b strings.Builder

	// Page header
	lastRefresh := ""
	if m.healthData != nil && !m.healthData.LastRefresh.IsZero() {
		lastRefresh = "Last refresh: " + m.healthData.LastRefresh.Format("15:04:05")
	}
	b.WriteString(PageHeader(
		fmt.Sprintf("Health · %s", m.selectedNetwork),
		lastRefresh,
	))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString(" Checking health...")
		return b.String()
	}

	if m.healthData == nil {
		b.WriteString(TextMuted.Render("  Press 'r' to run health checks"))
		return b.String()
	}

	h := m.healthData
	cardWidth := m.width - 6

	// Section 1: System Requirements
	if h.SystemHealth != nil {
		b.WriteString(m.renderSystemHealthCard(cardWidth))
		b.WriteString("\n\n")
	}

	// Section 2: Node Health (RPC endpoints)
	if h.NodeHealth != nil {
		b.WriteString(m.renderNodeHealthCard(cardWidth))
		b.WriteString("\n\n")

		// Check for network mismatch
		if h.NodeHealth.Height > 0 && !h.NodeHealth.ChainIDMatch {
			b.WriteString(NetworkMismatchWarning(string(m.selectedNetwork), "mismatched", cardWidth))
			b.WriteString("\n\n")
		}
	}

	// Section 3: Multi-node status (only if detected)
	if len(h.MultiNodeHealth) > 0 {
		hasData := false
		for _, n := range h.MultiNodeHealth {
			if n.Height > 0 {
				hasData = true
				break
			}
		}
		if hasData {
			b.WriteString(m.renderMultiNodeCard(cardWidth))
			b.WriteString("\n\n")
		}
	}

	// Section 4: Validator Health
	if h.ValidatorHealth != nil {
		b.WriteString(m.renderValidatorHealthCard(cardWidth))
	}

	return b.String()
}

func (m Model) renderSystemHealthCard(width int) string {
	h := m.healthData.SystemHealth

	// Build system info rows
	ramInfo := "N/A"
	if h.RAMTotal > 0 {
		ramInfo = fmt.Sprintf("%s total, %s free", formatBytes(h.RAMTotal), formatBytes(h.RAMFree))
	}

	diskInfo := "N/A"
	if h.DiskFree > 0 {
		diskInfo = formatBytes(h.DiskFree) + " free"
	}

	cpuInfo := "N/A"
	if h.CPUCount > 0 {
		cpuInfo = fmt.Sprintf("%d cores", h.CPUCount)
	}

	rows := [][]string{
		{"OS / Arch", fmt.Sprintf("%s / %s", h.OS, h.Arch)},
		{"CPU", cpuInfo},
		{"RAM", ramInfo},
		{"Disk", diskInfo},
	}

	body := Table(rows, 0)

	// Add ports section
	if len(h.Ports) > 0 {
		body += "\n" + TextMuted.Render("Ports:") + "\n"
		for _, p := range h.Ports {
			var status string
			if p.Listening {
				status = Badge(BadgeOK, "OPEN")
			} else {
				status = Badge(BadgeNA, "CLOSED")
			}
			body += fmt.Sprintf("  %-18s %d  %s\n", p.Name, p.Port, status)
		}
	}

	return Card("[1] System Requirements", body, width)
}

func (m Model) renderNodeHealthCard(width int) string {
	h := m.healthData.NodeHealth

	// RPC status table
	rows := []StatusRow{}

	addRPCRow := func(name string, s *RPCStatus) {
		if s == nil {
			rows = append(rows, StatusRow{Label: name, Status: BadgeNA, Note: "not checked"})
			return
		}
		status := BadgeOK
		if s.Status == "FAIL" {
			status = BadgeFail
		}
		note := s.Endpoint
		if s.Error != "" {
			note = s.Error
		}
		rows = append(rows, StatusRow{Label: name, Status: status, Note: truncateNote(note, 40)})
	}

	addRPCRow("Comet RPC", h.CometStatus)
	addRPCRow("Cosmos REST", h.CosmosStatus)
	addRPCRow("EVM JSON-RPC", h.EVMStatus)

	body := StatusTable(rows, 0)

	// Add sync info if available
	if h.Height > 0 {
		body += "\n"
		syncBadge := Badge(BadgeOK, "SYNCED")
		if h.CatchingUp {
			syncBadge = Badge(BadgeWarn, "SYNCING")
		}
		body += fmt.Sprintf("Height: %s  %s  Peers: %s\n",
			TextBright.Render(fmt.Sprintf("%d", h.Height)),
			syncBadge,
			TextBright.Render(fmt.Sprintf("%d", h.Peers)),
		)
	}

	return Card("[2] Node Health", body, width)
}

func (m Model) renderMultiNodeCard(width int) string {
	var body strings.Builder

	for _, n := range m.healthData.MultiNodeHealth {
		if n.Height == 0 {
			continue
		}
		syncBadge := Badge(BadgeOK, "SYNCED")
		if n.CatchingUp {
			syncBadge = Badge(BadgeWarn, "SYNCING")
		}
		body.WriteString(fmt.Sprintf("  %-10s height=%d %s\n", n.NodeName, n.Height, syncBadge))
	}

	return Card("Multi-Node Status", body.String(), width)
}

func (m Model) renderValidatorHealthCard(width int) string {
	h := m.healthData.ValidatorHealth

	if h.NotConfigured {
		body := TextMuted.Render("Validator not configured (no operator key detected)")
		return Card("[3] Validator Health", body, width)
	}

	rows := []StatusRow{
		{Label: "Valoper", Status: BadgeInfo, Value: truncateNote(h.ValoperAddr, 20)},
	}

	// Status badge
	statusBadge := BadgeNA
	switch h.Status {
	case "bonded":
		statusBadge = BadgeOK
	case "unbonding":
		statusBadge = BadgeWarn
	case "unbonded":
		statusBadge = BadgeFail
	}
	rows = append(rows, StatusRow{Label: "Status", Status: statusBadge, Value: strings.ToUpper(h.Status)})

	// Jailed status
	if h.Jailed {
		rows = append(rows, StatusRow{Label: "Jailed", Status: BadgeFail, Value: "YES"})
	} else {
		rows = append(rows, StatusRow{Label: "Jailed", Status: BadgeOK, Value: "NO"})
	}

	if h.MissedBlocks > 0 {
		rows = append(rows, StatusRow{Label: "Missed Blocks", Status: BadgeWarn, Value: fmt.Sprintf("%d", h.MissedBlocks)})
	}

	body := StatusTable(rows, 0)
	return Card("[3] Validator Health", body, width)
}

// Logs rendering with viewport
func (m Model) renderLogs() string {
	var b strings.Builder

	// Page header
	b.WriteString(PageHeader(fmt.Sprintf("Logs · %s", m.selectedNetwork), ""))
	b.WriteString("\n\n")

	// Config subview
	if m.subView == SubViewForm {
		return m.renderForm()
	}

	// Logs viewer with viewport
	if m.subView == SubViewLogsViewer && m.logsData.Streaming {
		// Status bar
		statusLine := fmt.Sprintf("  Streaming: %s / %s",
			TextBright.Render(m.logsData.Network),
			TextBright.Render(m.logsData.Service),
		)
		if m.logsData.Follow {
			statusLine += "  " + Badge(BadgeInfo, "FOLLOWING")
		}
		b.WriteString(statusLine)
		b.WriteString("\n\n")

		// Viewport for logs
		b.WriteString(m.viewport.View())
		b.WriteString("\n")
		b.WriteString(ScrollHint(m.viewport.AtTop(), m.viewport.AtBottom()))
		return b.String()
	}

	// Config display card
	l := m.logsData
	configRows := [][]string{
		{"Network", orValue(l.Network, string(m.selectedNetwork))},
		{"Service", orValue(l.Service, "monod")},
		{"Lines", fmt.Sprintf("%d", l.Lines)},
		{"Follow", fmt.Sprintf("%v", l.Follow)},
		{"Filter", orValue(l.Filter, "(none)")},
	}

	configBody := Table(configRows, 0)
	b.WriteString(Card("Configuration", configBody, m.width-6))
	b.WriteString("\n\n")

	// Actions
	actions := KeyHints(
		KeyHint("c", "configure"),
		KeyHint("s", "start streaming"),
		KeyHint("f", "toggle follow"),
	)
	b.WriteString(TextMuted.Render("  " + actions))

	// Show recent logs preview if any
	if len(l.LogLines) > 0 {
		b.WriteString("\n\n")
		b.WriteString(TextMuted.Render("  Recent logs (preview):"))
		b.WriteString("\n")
		start := len(l.LogLines) - 5
		if start < 0 {
			start = 0
		}
		for _, line := range l.LogLines[start:] {
			b.WriteString("  " + TextNormal.Render(truncate(line, m.width-6)) + "\n")
		}
	}

	return b.String()
}

// Update rendering with cards
func (m Model) renderUpdate() string {
	var b strings.Builder

	// Page header
	checkedAt := ""
	if m.updateData != nil && !m.updateData.CheckedAt.IsZero() {
		checkedAt = "Last check: " + m.updateData.CheckedAt.Format("15:04:05")
	}
	b.WriteString(PageHeader("Update", checkedAt))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString("  ")
		b.WriteString(m.spinner.View())
		b.WriteString(" Checking for updates...")
		return b.String()
	}

	if m.updateData == nil {
		b.WriteString(TextMuted.Render("  Press 'r' to check for updates"))
		return b.String()
	}

	u := m.updateData
	cardWidth := m.width - 6

	// Commander card
	cmdStatus := BadgeOK
	cmdStatusText := "Up to date"
	if u.CommanderUpdate {
		cmdStatus = BadgeWarn
		cmdStatusText = "Update available"
	}
	cmdRows := [][]string{
		{"Current", u.CommanderCurrent},
		{"Latest", u.CommanderLatest},
	}
	cmdBody := Table(cmdRows, 0)
	cmdBody += "\n" + StatusTable([]StatusRow{{Label: "Status", Status: cmdStatus, Value: cmdStatusText}}, 0)
	if u.CommanderUpdate {
		cmdBody += "\n" + TextAction.Render("Press 'u' to update")
	}
	b.WriteString(Card("Commander", cmdBody, cardWidth))
	b.WriteString("\n\n")

	// monod card
	monodStatus := BadgeNA
	if u.MonodCurrent != "" {
		monodStatus = BadgeOK
	}
	monodRows := [][]string{
		{"Current", orNA(u.MonodCurrent)},
		{"Latest", u.MonodLatest},
	}
	monodBody := Table(monodRows, 0)
	monodBody += "\n" + StatusTable([]StatusRow{{Label: "Status", Status: monodStatus}}, 0)
	b.WriteString(Card("monod", monodBody, cardWidth))
	b.WriteString("\n\n")

	// Sidecar card
	sidecarStatus := BadgeNA
	if u.SidecarCurrent != "" {
		sidecarStatus = BadgeOK
	}
	sidecarRows := [][]string{
		{"Current", orNA(u.SidecarCurrent)},
		{"Latest", u.SidecarLatest},
	}
	sidecarBody := Table(sidecarRows, 0)
	sidecarBody += "\n" + StatusTable([]StatusRow{{Label: "Status", Status: sidecarStatus}}, 0)
	b.WriteString(Card("Mesh/Rosetta Sidecar", sidecarBody, cardWidth))

	return b.String()
}

// Install rendering with wizard layout
func (m Model) renderInstall() string {
	var b strings.Builder

	// Page header
	b.WriteString(PageHeader("(Re)Install", ""))
	b.WriteString("\n\n")

	// Form subview
	if m.subView == SubViewForm {
		return m.renderForm()
	}

	i := m.installData

	// Wizard steps on the left
	steps := []WizardStep{
		{Number: 1, Title: "System Dependencies", Status: boolToStatus(i.DepsInstalled)},
		{Number: 2, Title: "Install monod", Status: boolToStatus(i.MonodVersion != "")},
		{Number: 3, Title: "Join Network", Status: statusFromString(i.JoinStatus)},
		{Number: 4, Title: "Install Mesh/Rosetta", Status: boolToStatus(i.MeshInstalled)},
	}

	// Calculate widths
	leftWidth := 35
	rightWidth := m.width - leftWidth - 8

	// Render wizard steps
	stepsContent := RenderWizardSteps(steps, leftWidth)
	stepsCard := Card("Wizard Steps", stepsContent, leftWidth)

	// Detail card on the right
	detailContent := m.renderInstallDetail()
	detailCard := Card("Details", detailContent, rightWidth)

	// Join horizontally
	if rightWidth > 20 {
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, stepsCard, "  ", detailCard))
	} else {
		b.WriteString(stepsCard)
		b.WriteString("\n")
		b.WriteString(detailCard)
	}

	b.WriteString("\n\n")

	// Actions
	actions := KeyHints(
		KeyHint("1", "deps"),
		KeyHint("2", "monod"),
		KeyHint("3", "join"),
		KeyHint("4", "mesh"),
	)
	b.WriteString(TextMuted.Render("  " + actions))

	return b.String()
}

func (m Model) renderInstallDetail() string {
	options := []struct {
		key  string
		name string
		desc string
	}{
		{"1", "System Dependencies", "Check and install curl, jq, etc."},
		{"2", "Install monod", "Install the Monolythium node binary"},
		{"3", "Join Network", "Download genesis, configure peers"},
		{"4", "Install Mesh/Rosetta", "Install the Rosetta API sidecar"},
	}

	var b strings.Builder
	for _, opt := range options {
		b.WriteString(TextAction.Render("["+opt.key+"]") + " " + TextBright.Render(opt.name) + "\n")
		b.WriteString("    " + TextMuted.Render(opt.desc) + "\n\n")
	}
	return b.String()
}

// Help rendering with viewport scroll
func (m Model) renderHelp() string {
	// Check if we need to initialize the help viewport
	if m.helpViewport.Width == 0 {
		// Return static content if viewport not initialized
		return m.renderHelpContent()
	}

	// Use the viewport
	return m.helpViewport.View() + "\n" + ScrollHint(m.helpViewport.AtTop(), m.helpViewport.AtBottom())
}

func (m Model) renderHelpContent() string {
	var b strings.Builder
	cardWidth := m.width - 6
	if cardWidth < 40 {
		cardWidth = 70
	}

	// What is Commander
	whatIs := `Mono Commander is a TUI-first tool for installing and operating
Monolythium nodes across Localnet, Sprintnet, Testnet, and Mainnet.

It provides a unified interface for node operators to manage their
infrastructure with safety constraints and best practices built-in.`
	b.WriteString(Card("What is Mono Commander?", whatIs, cardWidth))
	b.WriteString("\n\n")

	// Safety Constraints
	safety := `• ` + TextDanger.Render("No secrets stored") + `: Commander never stores mnemonics, private keys, or tokens
• ` + TextWarning.Render("No key generation") + `: Key management is handled by the node binary
• ` + TextWarning.Render("No rollback") + `: Emergency recovery is HALT → PATCH → UPGRADE → RESTART
• ` + TextSuccess.Render("Dry-run recommended") + `: Always preview changes before applying`
	b.WriteString(Card("Safety Constraints", safety, cardWidth))
	b.WriteString("\n\n")

	// Keyboard Shortcuts
	shortcuts := [][]string{
		{"Tab / Shift+Tab", "Switch between tabs"},
		{"← / →", "Navigate tabs"},
		{"Enter", "Select / Confirm"},
		{"Esc", "Go back / Cancel"},
		{"r", "Refresh current view"},
		{"q", "Quit"},
	}
	shortcutsBody := Table(shortcuts, 0)
	shortcutsBody += "\n" + TextMuted.Render("Tab-specific:") + "\n"
	shortcutsBody += "  Dashboard: " + TextAction.Render("n") + " = change network\n"
	shortcutsBody += "  Health: " + TextAction.Render("1/2/3") + " = section details\n"
	shortcutsBody += "  Logs: " + TextAction.Render("c") + " = config, " + TextAction.Render("s") + " = stream, " + TextAction.Render("f") + " = follow\n"
	shortcutsBody += "  Update: " + TextAction.Render("u") + " = update commander\n"
	shortcutsBody += "  Install: " + TextAction.Render("1/2/3/4") + " = wizard steps\n"
	b.WriteString(Card("Keyboard Shortcuts", shortcutsBody, cardWidth))
	b.WriteString("\n\n")

	// Mouse Support
	mouse := `• Click on tabs to switch
• Scroll wheel to navigate viewports
• Works alongside keyboard controls`
	b.WriteString(Card("Mouse Support", mouse, cardWidth))
	b.WriteString("\n\n")

	// Troubleshooting
	troubleshooting := TextMuted.Render("RPC unreachable:") + `
  • Check if the node is running: systemctl status monod
  • Verify the node is listening on expected ports

` + TextMuted.Render("Wrong chain-id / EVM chain-id:") + `
  • Ensure you're connected to the correct network
  • Check genesis.json matches the expected network

` + TextMuted.Render("Systemd not present:") + `
  • Systemd is required for service management on Linux
  • On macOS, use launchd or run manually

` + TextMuted.Render("Ports in use:") + `
  • Stop conflicting services: lsof -i :26657
  • Use different ports in config.toml`
	b.WriteString(Card("Troubleshooting", troubleshooting, cardWidth))
	b.WriteString("\n\n")

	// Links
	links := `• GitHub: https://github.com/monolythium/mono-commander
• Docs: https://docs.monolythium.com
• Issues: https://github.com/monolythium/mono-commander/issues`
	b.WriteString(Card("Documentation", links, cardWidth))

	return b.String()
}

// Form rendering (unchanged but with new styles)
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

	b.WriteString(PageHeader(title, ""))
	b.WriteString("\n\n")

	// Render form fields
	for i, field := range m.formFields {
		label := field.Label
		if field.Required {
			label += " " + TextDanger.Render("*")
		}

		style := TextMuted
		if i == m.formIndex {
			style = TextAction
		}

		b.WriteString("  ")
		b.WriteString(style.Render(label))
		b.WriteString("\n")
		b.WriteString("    ")
		b.WriteString(field.Input.View())
		b.WriteString("\n\n")
	}

	b.WriteString(TextMuted.Render("  Tab/↓: next • Shift+Tab/↑: prev • Enter: submit • Esc: cancel"))

	return b.String()
}

// Helper functions

func boolToStatus(b bool) BadgeType {
	if b {
		return BadgeOK
	}
	return BadgeFail
}

func serviceToStatus(s string) BadgeType {
	switch s {
	case "active", "running":
		return BadgeOK
	case "inactive":
		return BadgeNA
	case "failed":
		return BadgeFail
	default:
		return BadgeNA
	}
}

func statusFromString(s string) BadgeType {
	if s == "" {
		return BadgeNA
	}
	switch strings.ToLower(s) {
	case "ok", "success", "done":
		return BadgeOK
	case "warn", "warning":
		return BadgeWarn
	case "fail", "failed", "error":
		return BadgeFail
	default:
		return BadgeInfo
	}
}

func orEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func truncateNote(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func getExpectedChainID(network interface{}) string {
	switch fmt.Sprintf("%v", network) {
	case "Localnet":
		return "mono-local-1"
	case "Sprintnet":
		return "mono-sprint-1"
	case "Testnet":
		return "mono-test-1"
	case "Mainnet":
		return "mono-1"
	default:
		return ""
	}
}

// Keep existing helpers for backward compatibility
func formatStatus(installed bool, version string) string {
	if !installed {
		return Badge(BadgeFail, "NOT INSTALLED")
	}
	if version != "" {
		return Badge(BadgeOK, version)
	}
	return Badge(BadgeOK, "INSTALLED")
}

func formatBool(b bool) string {
	return BoolBadge(b)
}

func formatServiceStatus(status string) string {
	switch status {
	case "active":
		return Badge(BadgeOK, "ACTIVE")
	case "inactive":
		return Badge(BadgeNA, "INACTIVE")
	case "failed":
		return Badge(BadgeFail, "FAILED")
	case "not found", "not installed":
		return Badge(BadgeNA, "N/A")
	default:
		return Badge(BadgeNA, status)
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
	return Table(rows, indent)
}

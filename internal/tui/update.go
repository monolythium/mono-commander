package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/monolythium/mono-commander/internal/core"
	"github.com/monolythium/mono-commander/internal/logs"
	"github.com/monolythium/mono-commander/internal/mesh"
	"github.com/monolythium/mono-commander/internal/net"
	"github.com/monolythium/mono-commander/internal/update"
	"github.com/monolythium/mono-commander/internal/walletgen"
)

// Messages for async operations
type dashboardRefreshMsg struct {
	data *DashboardData
	err  error
}

type healthRefreshMsg struct {
	data *HealthData
	err  error
}

type logsLineMsg struct {
	line string
}

type logsErrorMsg struct {
	err error
}

type updateCheckMsg struct {
	data *UpdateData
	err  error
}

type updateApplyMsg struct {
	result *update.ApplyResult
	err    error
}

type networkChangedMsg struct {
	network core.NetworkName
}

type errMsg struct {
	err error
}

type walletGeneratedMsg struct {
	result *WalletResult
	err    error
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-10)

		viewportHeight := msg.Height - 10
		if viewportHeight < 5 {
			viewportHeight = 5
		}

		// Initialize or resize logs viewport
		if m.viewport.Width == 0 {
			m.viewport = NewViewport(msg.Width-4, viewportHeight)
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = viewportHeight
		}

		// Initialize or resize help viewport
		if m.helpViewport.Width == 0 {
			m.helpViewport = NewViewport(msg.Width-4, viewportHeight)
			m.helpViewport.SetContent(m.renderHelpContent())
		} else {
			m.helpViewport.Width = msg.Width - 4
			m.helpViewport.Height = viewportHeight
		}

		return m, nil

	case tea.MouseMsg:
		return m.handleMouseEvent(msg)

	case tea.KeyMsg:
		return m.handleKeypress(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case dashboardRefreshMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.status = fmt.Sprintf("Error: %s", msg.err.Error())
		} else {
			m.dashboardData = msg.data
			m.err = nil
			m.status = ""
		}
		return m, nil

	case healthRefreshMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.healthData = msg.data
			m.err = nil
		}
		return m, nil

	case logsLineMsg:
		if m.logsData != nil {
			m.logsData.LogLines = append(m.logsData.LogLines, msg.line)
			// Keep only last 1000 lines in memory
			if len(m.logsData.LogLines) > 1000 {
				m.logsData.LogLines = m.logsData.LogLines[len(m.logsData.LogLines)-1000:]
			}
			m.viewport.SetContent(strings.Join(m.logsData.LogLines, "\n"))
			m.viewport.GotoBottom()
		}
		return m, nil

	case logsErrorMsg:
		m.err = msg.err
		m.logsData.Streaming = false
		return m, nil

	case updateCheckMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.updateData = msg.data
			m.err = nil
		}
		return m, nil

	case updateApplyMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.status = fmt.Sprintf("Update failed: %v", msg.err)
		} else if msg.result != nil {
			if msg.result.Success {
				if msg.result.NewVersion != "" {
					m.status = fmt.Sprintf("Updated to %s. Please restart monoctl.", msg.result.NewVersion)
				} else {
					m.status = "Update installed. Please restart monoctl."
				}
				m.err = nil
			} else {
				m.status = fmt.Sprintf("Update failed: %s", msg.result.Error)
				if msg.result.NeedsSudo {
					m.status += " (needs sudo)"
				}
			}
		}
		return m, nil

	case networkChangedMsg:
		m.selectedNetwork = msg.network
		m.config.SelectedNetwork = string(msg.network)
		SaveConfig(m.config)
		return m, m.refreshDashboard()

	case walletGeneratedMsg:
		m.toolsData.WalletGenerating = false
		if msg.err != nil {
			m.toolsData.WalletError = msg.err
		} else {
			m.toolsData.WalletResult = msg.result
			m.toolsData.WalletError = nil
			m.subView = SubViewToolsWalletResult
			// Clear sensitive data
			m.toolsData.WalletPassword = ""
			m.toolsData.WalletConfirm = ""
		}
		return m, nil
	}

	// Update list if in list mode
	if m.subView == SubViewNone || m.subView == SubViewNetworkSelect {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport if viewing logs
	if m.activeTab == TabLogs && m.subView == SubViewLogsViewer {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit
	if key == "ctrl+c" {
		m.cancel()
		return m, tea.Quit
	}

	// Handle mode selection
	if m.subView == SubViewModeSelect {
		return m.handleModeSelectKey(msg)
	}

	// Handle form input if in form mode
	if m.subView == SubViewForm && len(m.formFields) > 0 {
		return m.handleFormInput(msg)
	}

	// Quit with 'q' (only when not in a subview)
	if key == "q" && m.subView == SubViewNone {
		m.cancel()
		return m, tea.Quit
	}

	// Back/Escape
	if key == "esc" && m.subView != SubViewNone {
		m.subView = SubViewNone
		m.formFields = nil
		m.status = ""
		return m, nil
	}

	// Tab navigation (only when not in a subview)
	if m.subView == SubViewNone {
		switch key {
		case "tab":
			m.activeTab = (m.activeTab + 1) % Tab(len(m.tabs))
			return m, m.onTabChange()
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + Tab(len(m.tabs))) % Tab(len(m.tabs))
			return m, m.onTabChange()
		case "left":
			if m.activeTab > 0 {
				m.activeTab--
				return m, m.onTabChange()
			}
		case "right":
			if int(m.activeTab) < len(m.tabs)-1 {
				m.activeTab++
				return m, m.onTabChange()
			}
		}
	}

	// Handle tab-specific keys
	switch m.activeTab {
	case TabDashboard:
		return m.handleDashboardKey(msg)
	case TabHealth:
		return m.handleHealthKey(msg)
	case TabLogs:
		return m.handleLogsKey(msg)
	case TabUpdate:
		return m.handleUpdateKey(msg)
	case TabInstall:
		return m.handleInstallKey(msg)
	case TabTools:
		return m.handleToolsKey(msg)
	case TabHelp:
		return m.handleHelpKey(msg)
	}

	return m, nil
}

func (m Model) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.formIndex = (m.formIndex + 1) % len(m.formFields)
		m = m.updateFormFocus()
		return m, nil

	case "shift+tab", "up":
		m.formIndex = (m.formIndex - 1 + len(m.formFields)) % len(m.formFields)
		m = m.updateFormFocus()
		return m, nil

	case "enter":
		// Collect form values
		for i, field := range m.formFields {
			m.formResult[field.Label] = m.formFields[i].Input.Value()
		}
		// Call the callback
		if m.formCallback != nil {
			return m.formCallback(m, m.formResult)
		}
		m.subView = SubViewNone
		return m, nil

	case "esc":
		m.subView = SubViewNone
		m.formFields = nil
		m.formResult = make(map[string]string)
		return m, nil
	}

	// Update the current input field
	if m.formIndex < len(m.formFields) {
		var cmd tea.Cmd
		m.formFields[m.formIndex].Input, cmd = m.formFields[m.formIndex].Input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleModeSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "q", "ctrl+c":
		m.cancel()
		return m, tea.Quit

	case "up", "k":
		if m.modeSelectIndex > 0 {
			m.modeSelectIndex--
		}
		return m, nil

	case "down", "j":
		if m.modeSelectIndex < len(m.modeSelectOptions)-1 {
			m.modeSelectIndex++
		}
		return m, nil

	case "enter", " ":
		// Save selected mode
		if m.modeSelectIndex < len(m.modeSelectOptions) {
			selectedMode := m.modeSelectOptions[m.modeSelectIndex]
			m.deploymentMode = selectedMode
			m.config.DeploymentMode = selectedMode
			SaveConfig(m.config)

			// Exit mode selection, return to dashboard
			m.subView = SubViewNone
			return m, m.refreshDashboard()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) updateFormFocus() Model {
	for i := range m.formFields {
		if i == m.formIndex {
			m.formFields[i].Input.Focus()
		} else {
			m.formFields[i].Input.Blur()
		}
	}
	return m
}

func (m Model) onTabChange() tea.Cmd {
	m.subView = SubViewNone
	m.status = ""
	m.err = nil

	switch m.activeTab {
	case TabDashboard:
		return m.refreshDashboard()
	case TabHealth:
		return m.refreshHealth()
	case TabUpdate:
		return m.checkUpdates()
	}
	return nil
}

// Dashboard commands
func (m Model) refreshDashboard() tea.Cmd {
	return func() tea.Msg {
		data := &DashboardData{
			LastRefresh: time.Now(),
		}

		// Check monod installation
		monodPath, err := exec.LookPath("monod")
		data.MonodInstalled = err == nil
		if data.MonodInstalled {
			if out, err := exec.Command(monodPath, "version").Output(); err == nil {
				data.MonodVersion = strings.TrimSpace(string(out))
			}
		}

		// Check home directory
		homeDir, _ := os.UserHomeDir()
		nodePath := homeDir + "/.monod"
		if _, err := os.Stat(nodePath); err == nil {
			data.HomeExists = true
		}

		// Check genesis
		genesisPath := nodePath + "/config/genesis.json"
		if _, err := os.Stat(genesisPath); err == nil {
			data.GenesisExists = true
		}

		// Get service status
		data.ServiceStatus = logs.GetSystemdServiceStatus(string(m.selectedNetwork))

		// Get mesh status
		if mesh.IsSystemdAvailable() {
			meshStatus := mesh.GetServiceStatus(string(m.selectedNetwork))
			if meshStatus != nil {
				data.MeshStatus = meshStatus.ActiveState
			} else {
				data.MeshStatus = "not installed"
			}
		}

		// Try to get node status
		endpoints := core.Endpoints{
			CometRPC:   "http://localhost:26657",
			CosmosREST: "http://localhost:1317",
			EVMRPC:     "http://localhost:8545",
		}
		opts := core.StatusOptions{
			Network:   m.selectedNetwork,
			Endpoints: endpoints,
		}
		if status, err := core.GetNodeStatus(opts); err == nil {
			data.NodeStatus = status
		}

		return dashboardRefreshMsg{data: data}
	}
}

// Health commands
func (m Model) refreshHealth() tea.Cmd {
	return func() tea.Msg {
		data := &HealthData{
			LastRefresh: time.Now(),
		}

		// System health
		data.SystemHealth = &SystemHealthInfo{
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
			CPUCount: runtime.NumCPU(),
		}

		// Get RAM info (best effort)
		data.SystemHealth.RAMTotal, data.SystemHealth.RAMFree = getMemoryInfo()

		// Get disk info (best effort)
		data.SystemHealth.DiskFree = getDiskFree()

		// Check ports
		data.SystemHealth.Ports = checkPorts(m.selectedNetwork)

		// Node health checks
		endpoints := core.Endpoints{
			CometRPC:   "http://localhost:26657",
			CosmosREST: "http://localhost:1317",
			EVMRPC:     "http://localhost:8545",
		}

		rpcResults := core.CheckRPC(m.selectedNetwork, endpoints)

		nodeHealth := &NodeHealthInfo{
			NodeName: string(m.selectedNetwork),
		}

		for _, r := range rpcResults.Results {
			status := &RPCStatus{
				Endpoint: r.Endpoint,
				Status:   r.Status,
				Details:  r.Details,
				Error:    r.Message,
			}
			switch r.Type {
			case "Comet RPC":
				nodeHealth.CometStatus = status
			case "Cosmos REST":
				nodeHealth.CosmosStatus = status
			case "EVM JSON-RPC":
				nodeHealth.EVMStatus = status
			}
		}

		// Get node status for height/peers
		opts := core.StatusOptions{
			Network:   m.selectedNetwork,
			Endpoints: endpoints,
		}
		if status, err := core.GetNodeStatus(opts); err == nil {
			nodeHealth.Height = status.LatestHeight
			nodeHealth.CatchingUp = status.CatchingUp
			nodeHealth.Peers = status.PeersCount
			nodeHealth.ChainIDMatch = true // We got a response
		}

		data.NodeHealth = nodeHealth

		// Validator health (placeholder - would need keyring access)
		data.ValidatorHealth = &ValidatorHealthInfo{
			NotConfigured: true,
		}

		// Check for multi-node setup
		homeDir, _ := os.UserHomeDir()
		multiNodeDirs := []string{"node1", "node2", "node3", "node4"}
		for _, dir := range multiNodeDirs {
			nodePath := homeDir + "/.mono-localnet/" + dir
			if _, err := os.Stat(nodePath); err == nil {
				// Multi-node setup detected
				data.MultiNodeHealth = append(data.MultiNodeHealth, NodeHealthInfo{
					NodeName: dir,
				})
			}
		}

		return healthRefreshMsg{data: data}
	}
}

// Logs commands
func (m Model) startLogsStream() tea.Cmd {
	return func() tea.Msg {
		ctx := m.ctx
		network := m.logsData.Network
		if network == "" {
			network = string(m.selectedNetwork)
		}

		homeDir, _ := os.UserHomeDir()
		home := homeDir + "/.monod"

		var source logs.LogSource
		var err error

		if m.logsData.Service == "mesh-rosetta" {
			opts := mesh.LogsOptions{
				Network: network,
				Follow:  m.logsData.Follow,
				Lines:   m.logsData.Lines,
			}
			source, err = mesh.GetLogSource(opts)
		} else {
			source, err = logs.GetLogSource(network, home, m.logsData.Follow, m.logsData.Lines)
		}

		if err != nil {
			return logsErrorMsg{err: err}
		}

		lines, err := source.Lines(ctx)
		if err != nil {
			return logsErrorMsg{err: err}
		}

		go func() {
			for line := range lines {
				// Filter if needed
				if m.logsData.Filter != "" {
					if !strings.Contains(strings.ToLower(line), strings.ToLower(m.logsData.Filter)) {
						continue
					}
				}
				// Send line to TUI (note: this is simplified, real impl would use program.Send)
			}
		}()

		return nil
	}
}

// Update commands
func (m Model) checkUpdates() tea.Cmd {
	return func() tea.Msg {
		data := &UpdateData{
			CommanderCurrent: Version,
			CheckedAt:        time.Now(),
			Checking:         false,
		}

		// Check monod version
		if monodPath, err := exec.LookPath("monod"); err == nil {
			if out, err := exec.Command(monodPath, "version").Output(); err == nil {
				data.MonodCurrent = strings.TrimSpace(string(out))
			}
		}

		// Check mesh-rosetta version
		if meshPath, err := exec.LookPath("mono-mesh-rosetta"); err == nil {
			if out, err := exec.Command(meshPath, "version").Output(); err == nil {
				data.SidecarCurrent = strings.TrimSpace(string(out))
			}
		}

		// Check GitHub releases for Commander
		client := update.NewClient()
		result, err := client.Check(Version)
		if err == nil && result != nil {
			data.CommanderLatest = result.LatestVersion
			data.CommanderUpdate = result.UpdateAvailable
		} else {
			data.CommanderLatest = Version
		}

		data.MonodLatest = "check manually"
		data.SidecarLatest = "check manually"

		return updateCheckMsg{data: data}
	}
}

// applyUpdate applies a Commander self-update
func (m Model) applyUpdate() tea.Cmd {
	return func() tea.Msg {
		client := update.NewClient()
		opts := update.ApplyOptions{
			CurrentVersion: Version,
			Yes:            true, // Skip confirmation in TUI
			Insecure:       false,
			DryRun:         false,
		}

		result, err := client.Apply(opts)
		return updateApplyMsg{result: result, err: err}
	}
}

// Helper functions
func getMemoryInfo() (total, free uint64) {
	// Platform-specific memory info (simplified)
	return 0, 0
}

func getDiskFree() uint64 {
	// Platform-specific disk info (simplified)
	return 0
}

func checkPorts(network core.NetworkName) []PortStatus {
	// Expected ports for the network
	ports := []PortStatus{
		{Name: "Comet RPC", Port: 26657},
		{Name: "Comet P2P", Port: 26656},
		{Name: "Cosmos REST", Port: 1317},
		{Name: "Cosmos gRPC", Port: 9090},
		{Name: "EVM JSON-RPC", Port: 8545},
		{Name: "EVM WebSocket", Port: 8546},
	}

	for i := range ports {
		ports[i].Listening = checkPortOpen(ports[i].Port)
	}

	return ports
}

func checkPortOpen(port int) bool {
	// Simplified port check
	return false
}

func newInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.Width = 50
	return ti
}

// NewViewport creates a new viewport
func NewViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))
	return vp
}

// Key handlers for each tab

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "n":
		// Open network selector
		m.subView = SubViewNetworkSelect
		return m.setupNetworkSelector(), nil
	case "r":
		// Refresh dashboard
		m.loading = true
		return m, m.refreshDashboard()
	case "enter":
		if m.subView == SubViewNetworkSelect {
			if selected, ok := m.list.SelectedItem().(MenuItem); ok {
				network, err := core.ParseNetworkName(selected.action)
				if err == nil {
					return m, func() tea.Msg {
						return networkChangedMsg{network: network}
					}
				}
			}
			m.subView = SubViewNone
		}
	case "esc":
		if m.subView != SubViewNone {
			m.subView = SubViewNone
		}
	}
	return m, nil
}

func (m Model) setupNetworkSelector() Model {
	items := make([]list.Item, len(m.networks))
	for i, n := range m.networks {
		items[i] = MenuItem{
			title:       string(n.Name),
			description: fmt.Sprintf("Chain ID: %s, EVM ID: %d", n.ChainID, n.EVMChainID),
			action:      string(n.Name),
		}
	}
	m.list.SetItems(items)
	m.list.Title = "Select Network"
	return m
}

func (m Model) handleHealthKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		m.loading = true
		return m, m.refreshHealth()
	case "1":
		m.subView = SubViewHealthSystem
	case "2":
		m.subView = SubViewHealthNode
	case "3":
		m.subView = SubViewHealthValidator
	case "esc":
		if m.subView != SubViewNone {
			m.subView = SubViewNone
		}
	}
	return m, nil
}

func (m Model) handleLogsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		// Configure logs
		m.subView = SubViewLogsConfig
		return m.setupLogsConfig(), nil
	case "f":
		// Toggle follow
		m.logsData.Follow = !m.logsData.Follow
	case "s":
		// Start/stop streaming
		if m.logsData.Streaming {
			m.logsData.Streaming = false
			m.cancel()
			m.ctx, m.cancel = context.WithCancel(context.Background())
		} else {
			m.logsData.Streaming = true
			m.logsData.Network = string(m.selectedNetwork)
			m.subView = SubViewLogsViewer
			return m, m.startLogsStream()
		}
	case "esc":
		if m.subView != SubViewNone {
			m.logsData.Streaming = false
			m.subView = SubViewNone
		}
	}
	return m, nil
}

func (m Model) setupLogsConfig() Model {
	m.formFields = []FormField{
		{Label: "network", Placeholder: string(m.selectedNetwork), Required: true, Input: newInput("Network")},
		{Label: "service", Placeholder: "monod", Required: true, Input: newInput("Service (monod/mesh-rosetta)")},
		{Label: "lines", Placeholder: "50", Required: false, Input: newInput("Number of lines")},
		{Label: "filter", Placeholder: "", Required: false, Input: newInput("Filter (optional)")},
	}
	m.formFields[0].Input.SetValue(string(m.selectedNetwork))
	m.formFields[1].Input.SetValue("monod")
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	m.subView = SubViewForm
	m.formCallback = func(m Model, result map[string]string) (Model, tea.Cmd) {
		m.logsData.Network = result["network"]
		m.logsData.Service = result["service"]
		if result["lines"] != "" {
			// Parse lines count
		}
		m.logsData.Filter = result["filter"]
		m.subView = SubViewNone
		return m, nil
	}
	return m
}

func (m Model) handleUpdateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		m.loading = true
		return m, m.checkUpdates()
	case "u":
		// Apply Commander update
		if m.updateData != nil && m.updateData.CommanderUpdate {
			m.loading = true
			m.status = "Updating Commander..."
			return m, m.applyUpdate()
		}
		m.status = "No update available"
	}
	return m, nil
}

func (m Model) handleInstallKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.subView = SubViewInstallDeps
	case "2":
		m.subView = SubViewInstallMonod
		return m.setupInstallMonodForm(), nil
	case "3":
		m.subView = SubViewInstallJoin
		return m.setupJoinForm(), nil
	case "4":
		m.subView = SubViewInstallRole
		return m.setupRoleForm(), nil
	case "5":
		m.subView = SubViewInstallMesh
		return m.setupMeshInstallForm(), nil
	case "esc":
		if m.subView != SubViewNone {
			m.subView = SubViewNone
		}
	}
	return m, nil
}

func (m Model) setupInstallMonodForm() Model {
	m.formFields = []FormField{
		{Label: "source", Placeholder: "local", Required: true, Input: newInput("Source (local/url)")},
		{Label: "path", Placeholder: "/path/to/monod", Required: true, Input: newInput("Binary path or URL")},
		{Label: "sha256", Placeholder: "", Required: false, Input: newInput("SHA256 (required for URL)")},
	}
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	m.subView = SubViewForm
	m.formCallback = func(m Model, result map[string]string) (Model, tea.Cmd) {
		m.subView = SubViewNone
		m.loading = true
		return m, func() tea.Msg {
			// Placeholder for actual install
			m.status = fmt.Sprintf("monod install from %s: %s (dry-run - use CLI for actual install)", result["source"], result["path"])
			m.loading = false
			return nil
		}
	}
	return m
}

func (m Model) setupJoinForm() Model {
	m.formFields = []FormField{
		{Label: "network", Placeholder: string(m.selectedNetwork), Required: true, Input: newInput("Network")},
		{Label: "sync", Placeholder: "bootstrap", Required: false, Input: newInput("Sync strategy (bootstrap/default)")},
		{Label: "home", Placeholder: "~/.monod", Required: false, Input: newInput("Node home directory")},
		{Label: "genesis-sha256", Placeholder: "", Required: false, Input: newInput("Genesis SHA256 (optional)")},
	}
	m.formFields[0].Input.SetValue(string(m.selectedNetwork))
	m.formFields[1].Input.SetValue("bootstrap") // Recommend bootstrap as default
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	m.subView = SubViewForm
	m.formCallback = func(m Model, result map[string]string) (Model, tea.Cmd) {
		m.subView = SubViewNone
		m.loading = true
		networkName := result["network"]
		syncStrategy := result["sync"]
		home := result["home"]
		if home == "" {
			homeDir, _ := os.UserHomeDir()
			home = homeDir + "/.monod"
		}
		return m, func() tea.Msg {
			network, err := core.ParseNetworkName(networkName)
			if err != nil {
				return dashboardRefreshMsg{err: err}
			}
			// Determine sync strategy
			strategy := core.SyncStrategyDefault
			if syncStrategy == "bootstrap" || syncStrategy == "" {
				strategy = core.SyncStrategyBootstrap
			}
			opts := core.JoinOptions{
				Network:       network,
				Home:          home,
				SyncStrategy:  strategy,
				ClearAddrbook: strategy == core.SyncStrategyBootstrap,
			}
			fetcher := &net.HTTPFetcher{}
			joinResult, err := core.Join(opts, fetcher)
			if err != nil {
				return dashboardRefreshMsg{err: err}
			}
			if !joinResult.Success {
				return dashboardRefreshMsg{err: fmt.Errorf("join failed")}
			}
			// Store sync strategy in install data
			if m.installData != nil {
				m.installData.SyncStrategy = string(strategy)
			}
			return dashboardRefreshMsg{}
		}
	}
	return m
}

func (m Model) setupRoleForm() Model {
	// Role selection form with clear descriptions
	m.formFields = []FormField{
		{Label: "role", Placeholder: "full_node", Required: true, Input: newInput("Role (full_node/archive_node/seed_node)")},
		{Label: "home", Placeholder: "~/.monod", Required: false, Input: newInput("Node home directory")},
	}
	m.formFields[0].Input.SetValue("full_node")
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	m.subView = SubViewForm
	m.formCallback = func(m Model, result map[string]string) (Model, tea.Cmd) {
		m.subView = SubViewNone
		m.loading = true
		roleStr := result["role"]
		home := result["home"]
		if home == "" {
			homeDir, _ := os.UserHomeDir()
			home = homeDir + "/.monod"
		}
		return m, func() tea.Msg {
			role, err := core.ParseNodeRole(roleStr)
			if err != nil {
				return dashboardRefreshMsg{err: err}
			}

			// Check if seed_node role is allowed
			if role == core.RoleSeedNode {
				allowed, reason := core.IsSeedModeAllowed(home)
				if !allowed {
					// We'll configure pruning=nothing anyway, but warn
					_ = reason // Log or display warning
				}
			}

			// Apply role configuration
			err = core.ApplyRoleConfig(home, role, false)
			if err != nil {
				return dashboardRefreshMsg{err: err}
			}
			return dashboardRefreshMsg{}
		}
	}
	return m
}

func (m Model) setupMeshInstallForm() Model {
	m.formFields = []FormField{
		{Label: "network", Placeholder: string(m.selectedNetwork), Required: true, Input: newInput("Network")},
		{Label: "url", Placeholder: "https://...", Required: true, Input: newInput("Download URL")},
		{Label: "sha256", Placeholder: "", Required: true, Input: newInput("SHA256 checksum")},
		{Label: "version", Placeholder: "v1.0.0", Required: false, Input: newInput("Version (optional)")},
	}
	m.formFields[0].Input.SetValue(string(m.selectedNetwork))
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	m.subView = SubViewForm
	m.formCallback = func(m Model, result map[string]string) (Model, tea.Cmd) {
		m.subView = SubViewNone
		m.loading = true
		url := result["url"]
		sha256 := result["sha256"]
		version := result["version"]
		return m, func() tea.Msg {
			opts := mesh.InstallOptions{
				URL:     url,
				SHA256:  sha256,
				Version: version,
			}
			installResult := mesh.Install(opts)
			if !installResult.Success {
				return dashboardRefreshMsg{err: installResult.Error}
			}
			return dashboardRefreshMsg{}
		}
	}
	return m
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle scroll keys in help tab
	switch msg.String() {
	case "up", "k":
		m.helpViewport.LineUp(1)
	case "down", "j":
		m.helpViewport.LineDown(1)
	case "pgup":
		m.helpViewport.HalfViewUp()
	case "pgdown", " ":
		m.helpViewport.HalfViewDown()
	case "home":
		m.helpViewport.GotoTop()
	case "end":
		m.helpViewport.GotoBottom()
	}
	return m, nil
}

func (m Model) handleToolsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle wallet result view
	if m.subView == SubViewToolsWalletResult {
		if key == "esc" || key == "enter" {
			m.subView = SubViewNone
			m.toolsData.WalletResult = nil
			m.toolsData.WalletName = ""
		}
		return m, nil
	}

	// Handle wallet form
	if m.subView == SubViewToolsWalletGen {
		return m.handleWalletFormKey(msg)
	}

	// Main tools view
	switch key {
	case "w":
		// Open wallet generator form
		m.subView = SubViewToolsWalletGen
		m.toolsData.WalletFormIndex = 0
		m.toolsData.WalletName = ""
		m.toolsData.WalletPassword = ""
		m.toolsData.WalletConfirm = ""
		m.toolsData.WalletError = nil
		m.toolsData.WalletResult = nil
		return m, nil
	case "esc":
		if m.subView != SubViewNone {
			m.subView = SubViewNone
		}
	}
	return m, nil
}

func (m Model) handleWalletFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "tab", "down":
		m.toolsData.WalletFormIndex = (m.toolsData.WalletFormIndex + 1) % 3
		return m, nil

	case "shift+tab", "up":
		m.toolsData.WalletFormIndex = (m.toolsData.WalletFormIndex - 1 + 3) % 3
		return m, nil

	case "enter":
		// Validate and submit
		if err := m.validateWalletForm(); err != nil {
			m.toolsData.WalletError = err
			return m, nil
		}
		// Start generation
		m.toolsData.WalletGenerating = true
		m.toolsData.WalletError = nil
		return m, m.generateWallet()

	case "esc":
		m.subView = SubViewNone
		m.toolsData.WalletName = ""
		m.toolsData.WalletPassword = ""
		m.toolsData.WalletConfirm = ""
		m.toolsData.WalletError = nil
		return m, nil

	case "backspace":
		// Delete character from current field
		switch m.toolsData.WalletFormIndex {
		case 0:
			if len(m.toolsData.WalletName) > 0 {
				m.toolsData.WalletName = m.toolsData.WalletName[:len(m.toolsData.WalletName)-1]
			}
		case 1:
			if len(m.toolsData.WalletPassword) > 0 {
				m.toolsData.WalletPassword = m.toolsData.WalletPassword[:len(m.toolsData.WalletPassword)-1]
			}
		case 2:
			if len(m.toolsData.WalletConfirm) > 0 {
				m.toolsData.WalletConfirm = m.toolsData.WalletConfirm[:len(m.toolsData.WalletConfirm)-1]
			}
		}
		return m, nil

	default:
		// Type character into current field
		if len(key) == 1 {
			r := rune(key[0])
			if r >= 32 && r < 127 { // Printable ASCII
				switch m.toolsData.WalletFormIndex {
				case 0:
					m.toolsData.WalletName += key
				case 1:
					m.toolsData.WalletPassword += key
				case 2:
					m.toolsData.WalletConfirm += key
				}
			}
		}
		return m, nil
	}
}

func (m Model) validateWalletForm() error {
	if len(m.toolsData.WalletPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if m.toolsData.WalletPassword != m.toolsData.WalletConfirm {
		return fmt.Errorf("passwords do not match")
	}
	return nil
}

func (m Model) generateWallet() tea.Cmd {
	name := m.toolsData.WalletName
	password := m.toolsData.WalletPassword

	return func() tea.Msg {
		// Generate keypair
		kp, err := walletgen.GenerateKeypair()
		if err != nil {
			return walletGeneratedMsg{err: fmt.Errorf("failed to generate keypair: %w", err)}
		}

		// Create keystore
		ks, err := walletgen.CreateKeystore(kp, password)
		if err != nil {
			return walletGeneratedMsg{err: fmt.Errorf("failed to create keystore: %w", err)}
		}

		// Get wallet directory
		walletDir, err := walletgen.GetDefaultWalletDir()
		if err != nil {
			return walletGeneratedMsg{err: fmt.Errorf("failed to get wallet directory: %w", err)}
		}

		// Generate filename
		evmAddr := kp.EVMAddress()
		filename := walletgen.GenerateKeystoreFilename(name, evmAddr)
		keystorePath := walletDir + "/" + filename

		// Save keystore
		if err := walletgen.SaveKeystore(ks, keystorePath); err != nil {
			return walletGeneratedMsg{err: fmt.Errorf("failed to save keystore: %w", err)}
		}

		// Get bech32 address
		bech32Addr, err := kp.Bech32Address()
		if err != nil {
			bech32Addr = "error: " + err.Error()
		}

		return walletGeneratedMsg{
			result: &WalletResult{
				KeystorePath:  keystorePath,
				EVMAddress:    evmAddr,
				Bech32Address: bech32Addr,
			},
		}
	}
}

// handleMouseEvent handles mouse events for tab switching and scrolling
func (m Model) handleMouseEvent(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.MouseLeft:
		// Check if click is in the tab bar area (first 2 lines)
		if msg.Y < 2 {
			// Calculate which tab was clicked
			_, positions := RenderTabBar(m.tabs, m.activeTab, m.width)
			clickedTab := positions.GetTabAtPosition(msg.X)
			if clickedTab >= 0 && clickedTab < Tab(len(m.tabs)) {
				m.activeTab = clickedTab
				m.subView = SubViewNone
				return m, m.onTabChange()
			}
		}

	case tea.MouseWheelUp:
		// Scroll up in appropriate viewport
		if m.activeTab == TabHelp {
			m.helpViewport.LineUp(3)
		} else if m.activeTab == TabLogs && m.subView == SubViewLogsViewer {
			m.viewport.LineUp(3)
		}

	case tea.MouseWheelDown:
		// Scroll down in appropriate viewport
		if m.activeTab == TabHelp {
			m.helpViewport.LineDown(3)
		} else if m.activeTab == TabLogs && m.subView == SubViewLogsViewer {
			m.viewport.LineDown(3)
		}
	}

	return m, nil
}

// Package tui provides the Bubble Tea TUI for mono-commander.
package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/monolythium/mono-commander/internal/core"
)

// Version is set at build time via ldflags
var Version = "dev"

// Tab represents a top-level tab
type Tab int

const (
	TabDashboard Tab = iota
	TabHealth
	TabLogs
	TabUpdate
	TabInstall
	TabTools
	TabHelp
)

func (t Tab) String() string {
	switch t {
	case TabDashboard:
		return "Dashboard"
	case TabHealth:
		return "Health"
	case TabLogs:
		return "Logs"
	case TabUpdate:
		return "Update"
	case TabInstall:
		return "(Re)Install"
	case TabTools:
		return "Tools"
	case TabHelp:
		return "Help"
	default:
		return "Unknown"
	}
}

// AllTabs returns all available tabs
func AllTabs() []Tab {
	return []Tab{TabDashboard, TabHealth, TabLogs, TabUpdate, TabInstall, TabTools, TabHelp}
}

// SubView represents a subview within a tab
type SubView int

const (
	SubViewNone SubView = iota
	// Mode selection (first-run)
	SubViewModeSelect
	// Dashboard subviews
	SubViewNetworkSelect
	// Health subviews
	SubViewHealthSystem
	SubViewHealthNode
	SubViewHealthValidator
	// Logs subviews
	SubViewLogsConfig
	SubViewLogsViewer
	// Update subviews
	SubViewUpdateCommander
	SubViewUpdateMonod
	SubViewUpdateSidecar
	// Install subviews
	SubViewInstallDeps
	SubViewInstallMonod
	SubViewInstallJoin
	SubViewInstallSync   // Sync strategy selection
	SubViewInstallRole
	SubViewInstallMesh
	// Tools subviews
	SubViewToolsWalletGen
	SubViewToolsWalletResult
	// Forms
	SubViewForm
)

// DeploymentMode represents the deployment method
type DeploymentMode string

const (
	DeployModeUnset      DeploymentMode = ""
	DeployModeHostNative DeploymentMode = "host-native"
	DeployModeDocker     DeploymentMode = "docker"
)

// Config holds user preferences
type Config struct {
	SelectedNetwork string         `json:"selected_network"`
	DeploymentMode  DeploymentMode `json:"deployment_mode"`
	LastUpdated     string         `json:"last_updated"`
}

// LoadConfig loads the user config from disk
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &Config{SelectedNetwork: "Sprintnet"}, nil
	}

	configPath := filepath.Join(homeDir, ".mono-commander", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return &Config{SelectedNetwork: "Sprintnet"}, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{SelectedNetwork: "Sprintnet"}, nil
	}

	return &cfg, nil
}

// SaveConfig saves the user config to disk
func SaveConfig(cfg *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".mono-commander")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	cfg.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)
}

// FormField represents a form input field
type FormField struct {
	Label       string
	Placeholder string
	Required    bool
	Input       textinput.Model
}

// MenuItem represents a menu item
type MenuItem struct {
	title       string
	description string
	action      string
}

func (i MenuItem) Title() string       { return i.title }
func (i MenuItem) Description() string { return i.description }
func (i MenuItem) FilterValue() string { return i.title }

// Model is the Bubble Tea model
type Model struct {
	// Tab navigation
	activeTab    Tab
	tabs         []Tab
	tabPositions TabPositions

	// Screen dimensions
	width  int
	height int

	// Deployment mode selection
	deploymentMode    DeploymentMode
	modeSelectIndex   int
	modeSelectOptions []DeploymentMode

	// User config
	config          *Config
	selectedNetwork core.NetworkName
	networks        []core.Network

	// Shared components
	list         list.Model
	spinner      spinner.Model
	viewport     viewport.Model
	helpViewport viewport.Model

	// Form state
	formFields   []FormField
	formIndex    int
	formResult   map[string]string
	formCallback func(Model, map[string]string) (Model, tea.Cmd)

	// Current subview
	subView SubView

	// Status/loading state
	status  string
	loading bool
	err     error

	// Dashboard state
	dashboardData *DashboardData

	// Health state
	healthData *HealthData

	// Logs state
	logsData *LogsData

	// Update state
	updateData *UpdateData

	// Install state
	installData *InstallData

	// Tools state
	toolsData *ToolsData

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// DashboardData holds dashboard-specific state
type DashboardData struct {
	NodeStatus      *core.NodeStatus
	MonodInstalled  bool
	MonodVersion    string
	HomeExists      bool
	GenesisExists   bool
	ServiceStatus   string
	MeshStatus      string
	CommanderUpdate *UpdateInfo
	LastRefresh     time.Time
}

// HealthData holds health check results
type HealthData struct {
	SystemHealth    *SystemHealthInfo
	NodeHealth      *NodeHealthInfo
	ValidatorHealth *ValidatorHealthInfo
	MultiNodeHealth []NodeHealthInfo
	LastRefresh     time.Time
}

// SystemHealthInfo holds system requirements health
type SystemHealthInfo struct {
	OS       string
	Arch     string
	CPUCount int
	RAMTotal uint64
	RAMFree  uint64
	DiskFree uint64
	Ports    []PortStatus
}

// PortStatus holds port listening status
type PortStatus struct {
	Name      string
	Port      int
	Listening bool
}

// NodeHealthInfo holds node health check results
type NodeHealthInfo struct {
	NodeName     string
	CometStatus  *RPCStatus
	CosmosStatus *RPCStatus
	EVMStatus    *RPCStatus
	ChainIDMatch bool
	EVMIDMatch   bool
	Height       int64
	CatchingUp   bool
	Peers        int
}

// RPCStatus holds individual RPC endpoint status
type RPCStatus struct {
	Endpoint string
	Status   string // "PASS" or "FAIL"
	Details  string
	Error    string
}

// ValidatorHealthInfo holds validator-specific health info
type ValidatorHealthInfo struct {
	IsValidator   bool
	ValoperAddr   string
	Status        string // bonded/unbonding/unbonded
	Jailed        bool
	MissedBlocks  int64
	JailedUntil   time.Time
	NotConfigured bool
}

// LogsData holds log viewer state
type LogsData struct {
	Network   string
	Service   string // "monod" or "mesh-rosetta"
	Source    string // "journalctl" or "file"
	Lines     int
	Follow    bool
	Filter    string
	LogLines  []string
	Streaming bool
}

// UpdateData holds update checker state
type UpdateData struct {
	CommanderCurrent string
	CommanderLatest  string
	CommanderUpdate  bool
	MonodCurrent     string
	MonodLatest      string
	MonodUpdate      bool
	SidecarCurrent   string
	SidecarLatest    string
	SidecarUpdate    bool
	CheckedAt        time.Time
	Checking         bool
}

// UpdateInfo holds update check result
type UpdateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
}

// InstallData holds install wizard state
type InstallData struct {
	Step           int
	DepsInstalled  bool
	MonodPath      string
	MonodVersion   string
	JoinStatus     string
	SyncStrategy   string // default, bootstrap, statesync
	SelectedRole   string // full_node, archive_node, seed_node
	RoleConfigured bool
	MeshInstalled  bool
}

// ToolsData holds tools state including wallet generator
type ToolsData struct {
	// Wallet generator state
	WalletName       string
	WalletPassword   string
	WalletConfirm    string
	WalletGenerating bool
	WalletResult     *WalletResult
	WalletError      error
	// Form field index (0=name, 1=password, 2=confirm)
	WalletFormIndex int
}

// WalletResult holds the result of wallet generation
type WalletResult struct {
	KeystorePath  string
	EVMAddress    string
	Bech32Address string
}

// Legacy style aliases for backward compatibility
var (
	titleStyle   = HeaderStyle
	statusStyle  = TextMuted.Copy().MarginLeft(2)
	helpStyle    = TextMuted
	focusedStyle = TextAction
	blurredStyle = TextMuted
	warningStyle = TextWarning
	successStyle = TextSuccess
	errorStyle   = TextDanger
	boxStyle     = CardStyle
	sectionStyle = lipgloss.NewStyle().MarginTop(1).MarginBottom(1)
)

// NewModel creates a new TUI model
func NewModel() Model {
	// Load user config
	cfg, _ := LoadConfig()

	// Parse selected network
	network, err := core.ParseNetworkName(cfg.SelectedNetwork)
	if err != nil {
		network = core.NetworkSprintnet
	}

	// Setup spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create list delegate
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(nil, delegate, 0, 0)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Determine deployment mode options based on platform
	var modeOptions []DeploymentMode
	if runtime.GOOS == "darwin" {
		// macOS: only Docker mode available (no systemd)
		modeOptions = []DeploymentMode{DeployModeDocker}
	} else {
		// Linux: both modes available
		modeOptions = []DeploymentMode{DeployModeHostNative, DeployModeDocker}
	}

	// Check if deployment mode needs to be selected
	deployMode := cfg.DeploymentMode
	initialSubView := SubViewNone

	// If on macOS and mode is unset or host-native, force Docker
	if runtime.GOOS == "darwin" && (deployMode == DeployModeUnset || deployMode == DeployModeHostNative) {
		deployMode = DeployModeDocker
		cfg.DeploymentMode = DeployModeDocker
		SaveConfig(cfg)
	}

	// If mode is unset, show mode selection
	if deployMode == DeployModeUnset {
		initialSubView = SubViewModeSelect
	}

	return Model{
		activeTab:         TabDashboard,
		tabs:              AllTabs(),
		config:            cfg,
		selectedNetwork:   network,
		networks:          core.ListNetworks(),
		deploymentMode:    deployMode,
		modeSelectIndex:   0,
		modeSelectOptions: modeOptions,
		list:              l,
		spinner:           s,
		formResult:        make(map[string]string),
		subView:           initialSubView,
		ctx:               ctx,
		cancel:            cancel,
		dashboardData:     &DashboardData{},
		healthData:        &HealthData{},
		logsData:          &LogsData{Lines: 50},
		updateData:        &UpdateData{CommanderCurrent: Version},
		installData:       &InstallData{},
		toolsData:         &ToolsData{},
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.refreshDashboard(),
	)
}

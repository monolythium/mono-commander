// Package tui provides the Bubble Tea TUI for mono-commander.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/monolythium/mono-commander/internal/core"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			MarginLeft(2)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginLeft(2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1).
			MarginLeft(2)

	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			MarginLeft(2)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			MarginLeft(2)

	commandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			MarginLeft(4)
)

// MenuItem represents a menu item
type MenuItem struct {
	title       string
	description string
	action      string
}

func (i MenuItem) Title() string       { return i.title }
func (i MenuItem) Description() string { return i.description }
func (i MenuItem) FilterValue() string { return i.title }

// View represents the current view
type View int

const (
	ViewMain View = iota
	ViewNetworks
	ViewJoin
	ViewPeersUpdate
	ViewSystemd
	ViewStatus
	ViewRPCCheck
	ViewLogs
	// M4: Validator action views
	ViewValidatorActions
	ViewCreateValidator
	ViewDelegate
	ViewUnbond
	ViewRedelegate
	ViewWithdrawRewards
	ViewVote
)

// FormField represents a form input field
type FormField struct {
	Label       string
	Placeholder string
	Required    bool
	Input       textinput.Model
}

// Model is the Bubble Tea model
type Model struct {
	list          list.Model
	currentView   View
	width         int
	height        int
	status        string
	networks      []core.Network
	err           error
	// M4: Form state
	formFields    []FormField
	formIndex     int
	formResult    map[string]string
	generatedCmd  string
	showConfirm   bool
	confirmChoice int // 0 = Copy command, 1 = Run (if enabled), 2 = Back
}

// NewModel creates a new TUI model
func NewModel() Model {
	items := []list.Item{
		MenuItem{
			title:       "Status",
			description: "Check node status",
			action:      "status",
		},
		MenuItem{
			title:       "RPC Check",
			description: "Check RPC endpoint health",
			action:      "rpc-check",
		},
		MenuItem{
			title:       "Logs",
			description: "Tail node logs",
			action:      "logs",
		},
		MenuItem{
			title:       "Networks",
			description: "View supported networks",
			action:      "networks",
		},
		MenuItem{
			title:       "Join Network",
			description: "Download genesis and configure peers",
			action:      "join",
		},
		MenuItem{
			title:       "Update Peers",
			description: "Update peer list from registry",
			action:      "peers",
		},
		MenuItem{
			title:       "Systemd Install",
			description: "Generate systemd unit file",
			action:      "systemd",
		},
		MenuItem{
			title:       "Validator Actions",
			description: "Create validator, delegate, unbond, vote",
			action:      "validator-actions",
		},
		MenuItem{
			title:       "Exit",
			description: "Quit mono-commander",
			action:      "exit",
		},
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(items, delegate, 0, 0)
	l.Title = "Mono Commander"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle

	return Model{
		list:        l,
		currentView: ViewMain,
		networks:    core.ListNetworks(),
		formResult:  make(map[string]string),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		// Handle form input first
		if len(m.formFields) > 0 && m.currentView >= ViewCreateValidator {
			return m.handleFormInput(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			if m.currentView == ViewMain {
				return m, tea.Quit
			}
			// Return to main menu or parent menu
			if m.currentView == ViewValidatorActions {
				m.currentView = ViewMain
			} else if m.currentView >= ViewCreateValidator {
				m = m.resetForm()
				m.currentView = ViewValidatorActions
			} else {
				m.currentView = ViewMain
			}
			m.status = ""
			return m, nil

		case "enter":
			if m.currentView == ViewMain {
				selected, ok := m.list.SelectedItem().(MenuItem)
				if ok {
					return m.handleAction(selected.action)
				}
			} else if m.currentView == ViewValidatorActions {
				selected, ok := m.list.SelectedItem().(MenuItem)
				if ok {
					return m.handleValidatorAction(selected.action)
				}
			}

		case "esc":
			if m.currentView == ViewValidatorActions {
				m.currentView = ViewMain
				m = m.setupMainMenu()
				return m, nil
			}
			if m.currentView >= ViewCreateValidator {
				m = m.resetForm()
				m.currentView = ViewValidatorActions
				m = m.setupValidatorMenu()
				return m, nil
			}
			if m.currentView != ViewMain {
				m.currentView = ViewMain
				m.status = ""
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// handleFormInput handles input when in a form view
func (m Model) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		if m.showConfirm {
			m.confirmChoice = (m.confirmChoice + 1) % 3
		} else {
			m.formIndex = (m.formIndex + 1) % len(m.formFields)
			m = m.updateFormFocus()
		}
		return m, nil

	case "shift+tab", "up":
		if m.showConfirm {
			m.confirmChoice = (m.confirmChoice - 1 + 3) % 3
		} else {
			m.formIndex = (m.formIndex - 1 + len(m.formFields)) % len(m.formFields)
			m = m.updateFormFocus()
		}
		return m, nil

	case "enter":
		if m.showConfirm {
			return m.handleConfirmChoice()
		}
		// Submit form
		return m.submitForm()

	case "esc":
		if m.showConfirm {
			m.showConfirm = false
			m.generatedCmd = ""
			return m, nil
		}
		m = m.resetForm()
		m.currentView = ViewValidatorActions
		m = m.setupValidatorMenu()
		return m, nil
	}

	// Update the current input field
	if !m.showConfirm && m.formIndex < len(m.formFields) {
		var cmd tea.Cmd
		m.formFields[m.formIndex].Input, cmd = m.formFields[m.formIndex].Input.Update(msg)
		return m, cmd
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

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	// Collect form values
	for i, field := range m.formFields {
		m.formResult[field.Label] = m.formFields[i].Input.Value()
	}

	// Generate command based on current view
	cmd, err := m.generateCommand()
	if err != nil {
		m.status = fmt.Sprintf("Error: %s", err.Error())
		return m, nil
	}

	m.generatedCmd = cmd
	m.showConfirm = true
	m.confirmChoice = 0
	return m, nil
}

func (m Model) handleConfirmChoice() (tea.Model, tea.Cmd) {
	switch m.confirmChoice {
	case 0: // Copy command (just show it)
		m.status = "Command shown above. Copy and run in your terminal."
	case 1: // Run - but we default to dry-run, so just show message
		m.status = "To execute, use CLI with --execute flag"
	case 2: // Back
		m.showConfirm = false
		m.generatedCmd = ""
		return m, nil
	}
	return m, nil
}

func (m Model) resetForm() Model {
	m.formFields = nil
	m.formIndex = 0
	m.formResult = make(map[string]string)
	m.generatedCmd = ""
	m.showConfirm = false
	m.confirmChoice = 0
	m.status = ""
	return m
}

// handleAction handles menu item selection
func (m Model) handleAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "networks":
		m.currentView = ViewNetworks
		m.status = ""
		return m, nil

	case "join":
		m.currentView = ViewJoin
		m.status = "Join flow requires CLI mode. Use: monoctl join --network <network> --home <path>"
		return m, nil

	case "peers":
		m.currentView = ViewPeersUpdate
		m.status = "Peers update requires CLI mode. Use: monoctl peers update --network <network>"
		return m, nil

	case "systemd":
		m.currentView = ViewSystemd
		m.status = "Systemd install requires CLI mode. Use: monoctl systemd install --network <network> --user <user>"
		return m, nil

	case "status":
		m.currentView = ViewStatus
		m.status = "Status requires CLI mode. Use: monoctl status --network <network> [--json]"
		return m, nil

	case "rpc-check":
		m.currentView = ViewRPCCheck
		m.status = "RPC check requires CLI mode. Use: monoctl rpc check --network <network> [--json]"
		return m, nil

	case "logs":
		m.currentView = ViewLogs
		m.status = "Logs requires CLI mode. Use: monoctl logs --network <network> [-f] [-n 50]"
		return m, nil

	case "validator-actions":
		m.currentView = ViewValidatorActions
		m = m.setupValidatorMenu()
		return m, nil

	case "exit":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) setupMainMenu() Model {
	items := []list.Item{
		MenuItem{title: "Status", description: "Check node status", action: "status"},
		MenuItem{title: "RPC Check", description: "Check RPC endpoint health", action: "rpc-check"},
		MenuItem{title: "Logs", description: "Tail node logs", action: "logs"},
		MenuItem{title: "Networks", description: "View supported networks", action: "networks"},
		MenuItem{title: "Join Network", description: "Download genesis and configure peers", action: "join"},
		MenuItem{title: "Update Peers", description: "Update peer list from registry", action: "peers"},
		MenuItem{title: "Systemd Install", description: "Generate systemd unit file", action: "systemd"},
		MenuItem{title: "Validator Actions", description: "Create validator, delegate, unbond, vote", action: "validator-actions"},
		MenuItem{title: "Exit", description: "Quit mono-commander", action: "exit"},
	}
	m.list.SetItems(items)
	m.list.Title = "Mono Commander"
	return m
}

func (m Model) setupValidatorMenu() Model {
	items := []list.Item{
		MenuItem{
			title:       "Create Validator",
			description: "Register as a validator (includes 100k LYTH burn)",
			action:      "create-validator",
		},
		MenuItem{
			title:       "Delegate",
			description: "Delegate tokens to a validator",
			action:      "delegate",
		},
		MenuItem{
			title:       "Unbond",
			description: "Unbond tokens from a validator (3-day period)",
			action:      "unbond",
		},
		MenuItem{
			title:       "Redelegate",
			description: "Move delegation between validators",
			action:      "redelegate",
		},
		MenuItem{
			title:       "Withdraw Rewards",
			description: "Withdraw staking rewards or commission",
			action:      "withdraw-rewards",
		},
		MenuItem{
			title:       "Vote",
			description: "Vote on a governance proposal",
			action:      "vote",
		},
		MenuItem{
			title:       "Back",
			description: "Return to main menu",
			action:      "back",
		},
	}
	m.list.SetItems(items)
	m.list.Title = "Validator Actions"
	return m
}

func (m Model) handleValidatorAction(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "create-validator":
		m.currentView = ViewCreateValidator
		m = m.setupCreateValidatorForm()
		return m, nil

	case "delegate":
		m.currentView = ViewDelegate
		m = m.setupDelegateForm()
		return m, nil

	case "unbond":
		m.currentView = ViewUnbond
		m = m.setupUnbondForm()
		return m, nil

	case "redelegate":
		m.currentView = ViewRedelegate
		m = m.setupRedelegateForm()
		return m, nil

	case "withdraw-rewards":
		m.currentView = ViewWithdrawRewards
		m = m.setupWithdrawRewardsForm()
		return m, nil

	case "vote":
		m.currentView = ViewVote
		m = m.setupVoteForm()
		return m, nil

	case "back":
		m.currentView = ViewMain
		m = m.setupMainMenu()
		return m, nil
	}
	return m, nil
}

func newInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.Width = 50
	return ti
}

func (m Model) setupCreateValidatorForm() Model {
	m.formFields = []FormField{
		{Label: "from", Placeholder: "Key name (e.g., validator)", Required: true, Input: newInput("Key name")},
		{Label: "moniker", Placeholder: "Validator name", Required: true, Input: newInput("Validator name")},
		{Label: "amount", Placeholder: "100000000000000000000000alyth (100k LYTH)", Required: true, Input: newInput("Self-bond amount in alyth")},
		{Label: "min-self-delegation", Placeholder: "100000000000000000000000alyth", Required: true, Input: newInput("Min self-delegation in alyth")},
		{Label: "commission-rate", Placeholder: "0.10", Required: true, Input: newInput("Commission rate (0.10 = 10%)")},
		{Label: "network", Placeholder: "Sprintnet", Required: true, Input: newInput("Network name")},
	}
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	return m
}

func (m Model) setupDelegateForm() Model {
	m.formFields = []FormField{
		{Label: "from", Placeholder: "Key name", Required: true, Input: newInput("Key name")},
		{Label: "to", Placeholder: "monovaloper1...", Required: true, Input: newInput("Validator address")},
		{Label: "amount", Placeholder: "1000000000000000000alyth (1 LYTH)", Required: true, Input: newInput("Amount in alyth")},
		{Label: "network", Placeholder: "Sprintnet", Required: true, Input: newInput("Network name")},
	}
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	return m
}

func (m Model) setupUnbondForm() Model {
	m.formFields = []FormField{
		{Label: "from", Placeholder: "Key name", Required: true, Input: newInput("Key name")},
		{Label: "from-validator", Placeholder: "monovaloper1...", Required: true, Input: newInput("Validator address")},
		{Label: "amount", Placeholder: "1000000000000000000alyth", Required: true, Input: newInput("Amount in alyth")},
		{Label: "network", Placeholder: "Sprintnet", Required: true, Input: newInput("Network name")},
	}
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	return m
}

func (m Model) setupRedelegateForm() Model {
	m.formFields = []FormField{
		{Label: "from", Placeholder: "Key name", Required: true, Input: newInput("Key name")},
		{Label: "src", Placeholder: "monovaloper1... (source)", Required: true, Input: newInput("Source validator")},
		{Label: "dst", Placeholder: "monovaloper1... (destination)", Required: true, Input: newInput("Destination validator")},
		{Label: "amount", Placeholder: "1000000000000000000alyth", Required: true, Input: newInput("Amount in alyth")},
		{Label: "network", Placeholder: "Sprintnet", Required: true, Input: newInput("Network name")},
	}
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	return m
}

func (m Model) setupWithdrawRewardsForm() Model {
	m.formFields = []FormField{
		{Label: "from", Placeholder: "Key name", Required: true, Input: newInput("Key name")},
		{Label: "validator", Placeholder: "monovaloper1... (optional)", Required: false, Input: newInput("Specific validator (optional)")},
		{Label: "commission", Placeholder: "false", Required: false, Input: newInput("Include commission? (true/false)")},
		{Label: "network", Placeholder: "Sprintnet", Required: true, Input: newInput("Network name")},
	}
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	return m
}

func (m Model) setupVoteForm() Model {
	m.formFields = []FormField{
		{Label: "from", Placeholder: "Key name", Required: true, Input: newInput("Key name")},
		{Label: "proposal", Placeholder: "1", Required: true, Input: newInput("Proposal ID")},
		{Label: "option", Placeholder: "yes|no|abstain|no_with_veto", Required: true, Input: newInput("Vote option")},
		{Label: "network", Placeholder: "Sprintnet", Required: true, Input: newInput("Network name")},
	}
	m.formFields[0].Input.Focus()
	m.formIndex = 0
	return m
}

func (m Model) generateCommand() (string, error) {
	// Build CLI command from form data
	var parts []string
	parts = append(parts, "monoctl")

	switch m.currentView {
	case ViewCreateValidator:
		parts = append(parts, "validator", "create")
		parts = append(parts, "--from", m.formResult["from"])
		parts = append(parts, "--moniker", m.formResult["moniker"])
		parts = append(parts, "--amount", m.formResult["amount"])
		parts = append(parts, "--min-self-delegation", m.formResult["min-self-delegation"])
		parts = append(parts, "--commission-rate", m.formResult["commission-rate"])
		parts = append(parts, "--commission-max-rate", "0.20")
		parts = append(parts, "--commission-max-change-rate", "0.01")
		parts = append(parts, "--network", m.formResult["network"])

	case ViewDelegate:
		parts = append(parts, "stake", "delegate")
		parts = append(parts, "--from", m.formResult["from"])
		parts = append(parts, "--to", m.formResult["to"])
		parts = append(parts, "--amount", m.formResult["amount"])
		parts = append(parts, "--network", m.formResult["network"])

	case ViewUnbond:
		parts = append(parts, "stake", "unbond")
		parts = append(parts, "--from", m.formResult["from"])
		parts = append(parts, "--from-validator", m.formResult["from-validator"])
		parts = append(parts, "--amount", m.formResult["amount"])
		parts = append(parts, "--network", m.formResult["network"])

	case ViewRedelegate:
		parts = append(parts, "stake", "redelegate")
		parts = append(parts, "--from", m.formResult["from"])
		parts = append(parts, "--src", m.formResult["src"])
		parts = append(parts, "--dst", m.formResult["dst"])
		parts = append(parts, "--amount", m.formResult["amount"])
		parts = append(parts, "--network", m.formResult["network"])

	case ViewWithdrawRewards:
		parts = append(parts, "rewards", "withdraw")
		parts = append(parts, "--from", m.formResult["from"])
		if v := m.formResult["validator"]; v != "" {
			parts = append(parts, "--validator", v)
		}
		if m.formResult["commission"] == "true" {
			parts = append(parts, "--commission")
		}
		parts = append(parts, "--network", m.formResult["network"])

	case ViewVote:
		parts = append(parts, "gov", "vote")
		parts = append(parts, "--from", m.formResult["from"])
		parts = append(parts, "--proposal", m.formResult["proposal"])
		parts = append(parts, "--option", m.formResult["option"])
		parts = append(parts, "--network", m.formResult["network"])

	default:
		return "", fmt.Errorf("unknown action")
	}

	// Always include dry-run by default (safety)
	parts = append(parts, "--dry-run")

	return strings.Join(parts, " "), nil
}

// View renders the UI
func (m Model) View() string {
	var b strings.Builder

	switch m.currentView {
	case ViewNetworks:
		b.WriteString(m.renderNetworksView())
	case ViewValidatorActions:
		b.WriteString(m.list.View())
	case ViewCreateValidator, ViewDelegate, ViewUnbond, ViewRedelegate, ViewWithdrawRewards, ViewVote:
		b.WriteString(m.renderFormView())
	case ViewJoin, ViewPeersUpdate, ViewSystemd, ViewStatus, ViewRPCCheck, ViewLogs:
		b.WriteString(m.renderActionView())
	default:
		b.WriteString(m.list.View())
	}

	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(m.status))
	}

	help := helpStyle.Render("q: quit • esc: back • enter: select • tab: next field")
	b.WriteString("\n")
	b.WriteString(help)

	return b.String()
}

// renderNetworksView renders the networks list
func (m Model) renderNetworksView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Supported Networks"))
	b.WriteString("\n\n")

	// Header
	header := fmt.Sprintf("  %-12s %-15s %-10s %s", "NAME", "CHAIN ID", "EVM ID", "EVM HEX")
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(header))
	b.WriteString("\n")
	b.WriteString("  " + strings.Repeat("-", 50))
	b.WriteString("\n")

	for _, n := range m.networks {
		row := fmt.Sprintf("  %-12s %-15s %-10d %s", n.Name, n.ChainID, n.EVMChainID, n.EVMChainIDHex())
		b.WriteString(row)
		b.WriteString("\n")
	}

	return b.String()
}

// renderActionView renders the action placeholder view
func (m Model) renderActionView() string {
	var b strings.Builder

	title := ""
	switch m.currentView {
	case ViewJoin:
		title = "Join Network"
	case ViewPeersUpdate:
		title = "Update Peers"
	case ViewSystemd:
		title = "Systemd Install"
	case ViewStatus:
		title = "Node Status"
	case ViewRPCCheck:
		title = "RPC Health Check"
	case ViewLogs:
		title = "Node Logs"
	}

	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render(m.status))
	b.WriteString("\n")

	return b.String()
}

// renderFormView renders a form for validator actions
func (m Model) renderFormView() string {
	var b strings.Builder

	// Title
	title := ""
	switch m.currentView {
	case ViewCreateValidator:
		title = "Create Validator"
	case ViewDelegate:
		title = "Delegate Tokens"
	case ViewUnbond:
		title = "Unbond Tokens"
	case ViewRedelegate:
		title = "Redelegate Tokens"
	case ViewWithdrawRewards:
		title = "Withdraw Rewards"
	case ViewVote:
		title = "Governance Vote"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Show confirm dialog if we have a generated command
	if m.showConfirm {
		b.WriteString(successStyle.Render("Command Preview:"))
		b.WriteString("\n\n")
		b.WriteString(commandStyle.Render(m.generatedCmd))
		b.WriteString("\n\n")

		if m.currentView == ViewCreateValidator {
			b.WriteString(warningStyle.Render("WARNING: Validator creation includes a 100,000 LYTH burn."))
			b.WriteString("\n")
			b.WriteString(warningStyle.Render("This is non-refundable per Monolythium blueprint."))
			b.WriteString("\n\n")
		}

		// Confirmation options
		options := []string{"Copy Command", "Run with --execute", "Back"}
		for i, opt := range options {
			cursor := "  "
			style := blurredStyle
			if i == m.confirmChoice {
				cursor = "> "
				style = focusedStyle
			}
			b.WriteString(style.Render(cursor + opt))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(statusStyle.Render("Note: Run command with --execute flag to actually submit the transaction."))
		return b.String()
	}

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

		b.WriteString(style.Render(label))
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(field.Input.View())
		b.WriteString("\n\n")
	}

	b.WriteString(statusStyle.Render("Tab/↓: next field • Shift+Tab/↑: prev • Enter: submit • Esc: cancel"))
	b.WriteString("\n")

	return b.String()
}

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

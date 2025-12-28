// Package tui provides the Bubble Tea TUI for mono-commander.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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
)

// Model is the Bubble Tea model
type Model struct {
	list        list.Model
	currentView View
	width       int
	height      int
	status      string
	networks    []core.Network
	err         error
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
		switch msg.String() {
		case "q", "ctrl+c":
			if m.currentView == ViewMain {
				return m, tea.Quit
			}
			// Return to main menu
			m.currentView = ViewMain
			m.status = ""
			return m, nil

		case "enter":
			if m.currentView == ViewMain {
				selected, ok := m.list.SelectedItem().(MenuItem)
				if ok {
					return m.handleAction(selected.action)
				}
			}

		case "esc":
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

	case "exit":
		return m, tea.Quit
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	var b strings.Builder

	switch m.currentView {
	case ViewNetworks:
		b.WriteString(m.renderNetworksView())
	case ViewJoin, ViewPeersUpdate, ViewSystemd, ViewStatus, ViewRPCCheck, ViewLogs:
		b.WriteString(m.renderActionView())
	default:
		b.WriteString(m.list.View())
	}

	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(m.status))
	}

	help := helpStyle.Render("q: quit • esc: back • enter: select")
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

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

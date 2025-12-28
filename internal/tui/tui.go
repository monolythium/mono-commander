// Package tui provides the Bubble Tea TUI for mono-commander.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(
		NewModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Enable mouse support
	)
	_, err := p.Run()
	return err
}

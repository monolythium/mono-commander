package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	m := NewModel()

	if m.currentView != ViewMain {
		t.Errorf("NewModel() currentView = %v, want ViewMain", m.currentView)
	}

	if len(m.networks) != 4 {
		t.Errorf("NewModel() networks count = %d, want 4", len(m.networks))
	}
}

func TestModel_Update_Quit(t *testing.T) {
	m := NewModel()

	// Test 'q' key quits from main menu
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("Update('q') should return quit command")
	}

	// Verify model returned
	if _, ok := newModel.(Model); !ok {
		t.Error("Update should return Model type")
	}
}

func TestModel_Update_NavigateToNetworks(t *testing.T) {
	m := NewModel()

	// Simulate selecting "Networks" (first item)
	// First, we need to test the action handling directly
	newModel, _ := m.handleAction("networks")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewNetworks {
		t.Errorf("handleAction('networks') currentView = %v, want ViewNetworks", resultModel.currentView)
	}
}

func TestModel_Update_ReturnToMain(t *testing.T) {
	m := NewModel()
	m.currentView = ViewNetworks

	// Test 'q' returns to main when not on main
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewMain {
		t.Errorf("Update('q') from sub-view should return to main, got %v", resultModel.currentView)
	}
}

func TestModel_View(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// Main view should contain title
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}

	// Networks view
	m.currentView = ViewNetworks
	view = m.View()
	if view == "" {
		t.Error("View() networks returned empty string")
	}
}

func TestModel_Init(t *testing.T) {
	m := NewModel()
	cmd := m.Init()

	// Init should return nil (no initial command)
	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

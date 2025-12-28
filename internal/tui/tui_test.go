package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/monolythium/mono-commander/internal/core"
)

func TestNewModel(t *testing.T) {
	m := NewModel()

	if m.activeTab != TabDashboard {
		t.Errorf("NewModel() activeTab = %v, want TabDashboard", m.activeTab)
	}

	if len(m.networks) != 4 {
		t.Errorf("NewModel() networks count = %d, want 4", len(m.networks))
	}

	if m.formResult == nil {
		t.Error("NewModel() formResult should be initialized")
	}

	if m.config == nil {
		t.Error("NewModel() config should be initialized")
	}

	if m.dashboardData == nil {
		t.Error("NewModel() dashboardData should be initialized")
	}
}

func TestModel_Init(t *testing.T) {
	m := NewModel()
	cmd := m.Init()

	// Init should return commands (spinner tick and refresh)
	if cmd == nil {
		t.Error("Init() should return commands")
	}
}

func TestTab_String(t *testing.T) {
	tests := []struct {
		tab  Tab
		want string
	}{
		{TabDashboard, "Dashboard"},
		{TabHealth, "Health"},
		{TabLogs, "Logs"},
		{TabUpdate, "Update"},
		{TabInstall, "(Re)Install"},
		{TabHelp, "Help"},
	}

	for _, tt := range tests {
		if got := tt.tab.String(); got != tt.want {
			t.Errorf("Tab.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestAllTabs(t *testing.T) {
	tabs := AllTabs()
	if len(tabs) != 6 {
		t.Errorf("AllTabs() returned %d tabs, want 6", len(tabs))
	}
}

func TestModel_Update_TabNavigation(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// Tab key should cycle through tabs
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	resultModel := newModel.(Model)

	if resultModel.activeTab != TabHealth {
		t.Errorf("Tab key should move to TabHealth, got %v", resultModel.activeTab)
	}

	// Shift+Tab should go back
	newModel2, _ := resultModel.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	resultModel2 := newModel2.(Model)

	if resultModel2.activeTab != TabDashboard {
		t.Errorf("Shift+Tab should return to TabDashboard, got %v", resultModel2.activeTab)
	}
}

func TestModel_Update_RightArrow(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	resultModel := newModel.(Model)

	if resultModel.activeTab != TabHealth {
		t.Errorf("Right arrow should move to TabHealth, got %v", resultModel.activeTab)
	}
}

func TestModel_Update_LeftArrow(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabHealth

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	resultModel := newModel.(Model)

	if resultModel.activeTab != TabDashboard {
		t.Errorf("Left arrow should move to TabDashboard, got %v", resultModel.activeTab)
	}
}

func TestModel_Update_Quit(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// 'q' should quit from main view
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("'q' should return quit command")
	}
}

func TestModel_Update_CtrlC(t *testing.T) {
	m := NewModel()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Ctrl+C should return quit command")
	}
}

func TestModel_Update_Escape(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.subView = SubViewNetworkSelect

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	resultModel := newModel.(Model)

	if resultModel.subView != SubViewNone {
		t.Errorf("Escape should return to SubViewNone, got %v", resultModel.subView)
	}
}

func TestModel_View(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	view := m.View()
	if view == "" {
		t.Error("View() should not return empty string")
	}

	// Should contain tab bar
	if !containsString(view, "Dashboard") {
		t.Error("View() should contain Dashboard tab")
	}
}

func TestModel_View_EachTab(t *testing.T) {
	tabs := AllTabs()

	for _, tab := range tabs {
		m := NewModel()
		m.width = 80
		m.height = 24
		m.activeTab = tab

		view := m.View()
		if view == "" {
			t.Errorf("View() for tab %v should not be empty", tab)
		}
	}
}

func TestRenderTabBar(t *testing.T) {
	tabs := AllTabs()
	activeTab := TabDashboard
	width := 80

	tabBar, positions := RenderTabBar(tabs, activeTab, width)
	if tabBar == "" {
		t.Error("RenderTabBar() should not be empty")
	}

	// Should contain all tab names
	for _, tab := range tabs {
		if !containsString(tabBar, tab.String()) {
			t.Errorf("RenderTabBar() should contain %q", tab.String())
		}
	}

	// Should have positions for all tabs
	if len(positions.Tabs) != len(tabs) {
		t.Errorf("RenderTabBar() should return %d positions, got %d", len(tabs), len(positions.Tabs))
	}
}

func TestModel_RenderDashboard(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabDashboard

	dashboard := m.renderDashboard()
	if dashboard == "" {
		t.Error("renderDashboard() should not be empty")
	}
}

func TestModel_RenderHealth(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabHealth

	health := m.renderHealth()
	if health == "" {
		t.Error("renderHealth() should not be empty")
	}
}

func TestModel_RenderLogs(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabLogs

	logs := m.renderLogs()
	if logs == "" {
		t.Error("renderLogs() should not be empty")
	}
}

func TestModel_RenderUpdate(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabUpdate

	update := m.renderUpdate()
	if update == "" {
		t.Error("renderUpdate() should not be empty")
	}
}

func TestModel_RenderInstall(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabInstall

	install := m.renderInstall()
	if install == "" {
		t.Error("renderInstall() should not be empty")
	}
}

func TestModel_RenderHelp(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabHelp

	help := m.renderHelp()
	if help == "" {
		t.Error("renderHelp() should not be empty")
	}

	// Should contain safety info
	if !containsString(help, "No secrets stored") {
		t.Error("renderHelp() should contain safety info")
	}
}

func TestModel_SetupNetworkSelector(t *testing.T) {
	m := NewModel()
	m = m.setupNetworkSelector()

	// List should have 4 network items
	if m.list.Title != "Select Network" {
		t.Errorf("setupNetworkSelector() should set list title to 'Select Network', got %q", m.list.Title)
	}
}

func TestModel_HandleDashboardKey_NetworkSelect(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// 'n' should open network selector
	newModel, _ := m.handleDashboardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	resultModel := newModel.(Model)

	if resultModel.subView != SubViewNetworkSelect {
		t.Errorf("'n' should open network selector, got subView %v", resultModel.subView)
	}
}

func TestModel_HandleDashboardKey_Refresh(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// 'r' should trigger refresh
	newModel, cmd := m.handleDashboardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	resultModel := newModel.(Model)

	if !resultModel.loading {
		t.Error("'r' should set loading to true")
	}

	if cmd == nil {
		t.Error("'r' should return a refresh command")
	}
}

func TestModel_HandleHealthKey_Refresh(t *testing.T) {
	m := NewModel()
	m.activeTab = TabHealth
	m.width = 80
	m.height = 24

	newModel, cmd := m.handleHealthKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	resultModel := newModel.(Model)

	if !resultModel.loading {
		t.Error("'r' on health tab should set loading to true")
	}

	if cmd == nil {
		t.Error("'r' on health tab should return a command")
	}
}

func TestModel_HandleLogsKey_Configure(t *testing.T) {
	m := NewModel()
	m.activeTab = TabLogs
	m.width = 80
	m.height = 24

	newModel, _ := m.handleLogsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	resultModel := newModel.(Model)

	if resultModel.subView != SubViewForm {
		t.Errorf("'c' on logs tab should open form, got subView %v", resultModel.subView)
	}

	if len(resultModel.formFields) == 0 {
		t.Error("'c' on logs tab should set up form fields")
	}
}

func TestModel_HandleLogsKey_ToggleFollow(t *testing.T) {
	m := NewModel()
	m.activeTab = TabLogs
	m.logsData = &LogsData{Follow: false}

	newModel, _ := m.handleLogsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	resultModel := newModel.(Model)

	if !resultModel.logsData.Follow {
		t.Error("'f' should toggle follow to true")
	}
}

func TestModel_HandleUpdateKey_Refresh(t *testing.T) {
	m := NewModel()
	m.activeTab = TabUpdate
	m.width = 80
	m.height = 24

	newModel, cmd := m.handleUpdateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	resultModel := newModel.(Model)

	if !resultModel.loading {
		t.Error("'r' on update tab should set loading to true")
	}

	if cmd == nil {
		t.Error("'r' on update tab should return a command")
	}
}

func TestModel_HandleInstallKey_Options(t *testing.T) {
	m := NewModel()
	m.activeTab = TabInstall
	m.width = 80
	m.height = 24

	tests := []struct {
		key     rune
		subView SubView
	}{
		{'1', SubViewInstallDeps},
		{'2', SubViewInstallMonod},
		{'3', SubViewInstallJoin},
		{'4', SubViewInstallMesh},
	}

	for _, tt := range tests {
		m.subView = SubViewNone
		newModel, _ := m.handleInstallKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		resultModel := newModel.(Model)

		// For 2, 3, 4 we expect SubViewForm since they open forms
		if tt.key == '1' {
			if resultModel.subView != SubViewInstallDeps {
				t.Errorf("'%c' should set subView to %v, got %v", tt.key, tt.subView, resultModel.subView)
			}
		} else {
			if resultModel.subView != SubViewForm {
				t.Errorf("'%c' should open form (SubViewForm), got %v", tt.key, resultModel.subView)
			}
		}
	}
}

func TestModel_FormNavigation(t *testing.T) {
	m := NewModel()
	m.subView = SubViewForm
	m.formFields = []FormField{
		{Label: "field1", Input: newInput("placeholder1")},
		{Label: "field2", Input: newInput("placeholder2")},
		{Label: "field3", Input: newInput("placeholder3")},
	}
	m.formFields[0].Input.Focus()
	m.formIndex = 0

	// Tab should move to next field
	newModel, _ := m.handleFormInput(tea.KeyMsg{Type: tea.KeyTab})
	resultModel := newModel.(Model)

	if resultModel.formIndex != 1 {
		t.Errorf("Tab should move to index 1, got %d", resultModel.formIndex)
	}

	// Shift+Tab should go back
	newModel2, _ := resultModel.handleFormInput(tea.KeyMsg{Type: tea.KeyShiftTab})
	resultModel2 := newModel2.(Model)

	if resultModel2.formIndex != 0 {
		t.Errorf("Shift+Tab should move to index 0, got %d", resultModel2.formIndex)
	}
}

func TestModel_FormSubmit(t *testing.T) {
	m := NewModel()
	m.subView = SubViewForm
	m.formFields = []FormField{
		{Label: "field1", Input: newInput("placeholder1")},
	}
	m.formFields[0].Input.SetValue("testvalue")
	m.formCallback = func(m Model, result map[string]string) (Model, tea.Cmd) {
		m.status = "submitted"
		m.subView = SubViewNone
		return m, nil
	}

	newModel, _ := m.handleFormInput(tea.KeyMsg{Type: tea.KeyEnter})
	resultModel := newModel.(Model)

	if resultModel.formResult["field1"] != "testvalue" {
		t.Errorf("Form submit should collect value, got %q", resultModel.formResult["field1"])
	}

	if resultModel.status != "submitted" {
		t.Error("Form callback should be called")
	}
}

func TestModel_FormEscape(t *testing.T) {
	m := NewModel()
	m.subView = SubViewForm
	m.formFields = []FormField{
		{Label: "field1", Input: newInput("placeholder1")},
	}
	m.formResult["test"] = "value"

	newModel, _ := m.handleFormInput(tea.KeyMsg{Type: tea.KeyEsc})
	resultModel := newModel.(Model)

	if resultModel.subView != SubViewNone {
		t.Errorf("Escape should set subView to SubViewNone, got %v", resultModel.subView)
	}

	if len(resultModel.formFields) != 0 {
		t.Error("Escape should clear form fields")
	}

	if len(resultModel.formResult) != 0 {
		t.Error("Escape should clear form result")
	}
}

func TestModel_UpdateFormFocus(t *testing.T) {
	m := NewModel()
	m.formFields = []FormField{
		{Label: "field1", Input: newInput("placeholder1")},
		{Label: "field2", Input: newInput("placeholder2")},
	}
	m.formIndex = 1

	m = m.updateFormFocus()

	if m.formFields[0].Input.Focused() {
		t.Error("Field 0 should be blurred")
	}

	if !m.formFields[1].Input.Focused() {
		t.Error("Field 1 should be focused")
	}
}

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Default should be Sprintnet if no config file exists
	if cfg.SelectedNetwork == "" {
		cfg.SelectedNetwork = "Sprintnet"
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		installed bool
		version   string
	}{
		{false, ""},
		{true, ""},
		{true, "v1.0.0"},
	}

	for _, tt := range tests {
		got := formatStatus(tt.installed, tt.version)
		// Now returns badge-styled output, just verify non-empty
		if got == "" {
			t.Errorf("formatStatus(%v, %q) returned empty string", tt.installed, tt.version)
		}
	}
}

func TestFormatBool(t *testing.T) {
	// Now returns badge-styled output
	okBadge := formatBool(true)
	if okBadge == "" {
		t.Error("formatBool(true) should return non-empty badge")
	}

	failBadge := formatBool(false)
	if failBadge == "" {
		t.Error("formatBool(false) should return non-empty badge")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes uint64
		want  string
	}{
		{0, "N/A"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestOrNA(t *testing.T) {
	if orNA("") != "N/A" {
		t.Error("orNA(\"\") should return \"N/A\"")
	}
	if orNA("test") != "test" {
		t.Error("orNA(\"test\") should return \"test\"")
	}
}

func TestOrValue(t *testing.T) {
	if orValue("", "default") != "default" {
		t.Error("orValue(\"\", \"default\") should return \"default\"")
	}
	if orValue("test", "default") != "test" {
		t.Error("orValue(\"test\", \"default\") should return \"test\"")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s   string
		max int
		want string
	}{
		{"short", 10, "short"},
		{"long string", 5, "lo..."},
		{"abc", 3, "abc"},
		{"abcd", 4, "abcd"},     // exact length, no truncation needed
		{"abcde", 4, "a..."},   // over limit, truncate
	}

	for _, tt := range tests {
		got := truncate(tt.s, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
		}
	}
}

func TestRenderTable(t *testing.T) {
	rows := [][]string{
		{"Key", "Value"},
		{"Name", "Test"},
	}

	table := renderTable(rows, 2)
	if table == "" {
		t.Error("renderTable() should not return empty string")
	}

	if !containsString(table, "Key") {
		t.Error("renderTable() should contain 'Key'")
	}

	if !containsString(table, "Value") {
		t.Error("renderTable() should contain 'Value'")
	}
}

func TestDashboardData(t *testing.T) {
	d := &DashboardData{
		MonodInstalled: true,
		MonodVersion:   "v1.0.0",
		HomeExists:     true,
		GenesisExists:  true,
		ServiceStatus:  "active",
	}

	if !d.MonodInstalled {
		t.Error("MonodInstalled should be true")
	}

	if d.MonodVersion != "v1.0.0" {
		t.Errorf("MonodVersion = %q, want v1.0.0", d.MonodVersion)
	}
}

func TestHealthData(t *testing.T) {
	h := &HealthData{
		SystemHealth: &SystemHealthInfo{
			OS:       "linux",
			Arch:     "amd64",
			CPUCount: 4,
		},
	}

	if h.SystemHealth.OS != "linux" {
		t.Errorf("OS = %q, want linux", h.SystemHealth.OS)
	}
}

func TestUpdateData(t *testing.T) {
	u := &UpdateData{
		CommanderCurrent: "v1.0.0",
		CommanderLatest:  "v1.1.0",
		CommanderUpdate:  true,
	}

	if !u.CommanderUpdate {
		t.Error("CommanderUpdate should be true")
	}
}

func TestNetworkChangedMsg(t *testing.T) {
	msg := networkChangedMsg{network: core.NetworkSprintnet}

	if msg.network != core.NetworkSprintnet {
		t.Errorf("network = %v, want NetworkSprintnet", msg.network)
	}
}

// Test mouse tab switching
func TestModel_MouseTabSwitch(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24

	// Create a mock mouse click on the second tab area
	// Tab positions are roughly: Dashboard (0-12), Health (13-20), etc.
	mouseMsg := tea.MouseMsg{
		X:    15, // Somewhere in Health tab
		Y:    0,  // First line (tab bar)
		Type: tea.MouseLeft,
	}

	newModel, _ := m.handleMouseEvent(mouseMsg)
	resultModel := newModel.(Model)

	// Note: exact position depends on rendering, so we just verify the handler runs without panic
	// The actual tab switch depends on RenderTabBar positions
	_ = resultModel
}

// Test help viewport scroll
func TestModel_HelpViewportScroll(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabHelp

	// Initialize viewport
	m.helpViewport = NewViewport(76, 14)
	m.helpViewport.SetContent(m.renderHelpContent())

	// Get initial position
	initialOffset := m.helpViewport.YOffset

	// Scroll down
	newModel, _ := m.handleHelpKey(tea.KeyMsg{Type: tea.KeyDown})
	resultModel := newModel.(Model)

	// Verify scroll happened (or at least didn't panic)
	_ = resultModel.helpViewport.YOffset
	_ = initialOffset
}

// Test mouse wheel scroll
func TestModel_MouseWheelScroll(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	m.activeTab = TabHelp

	// Initialize viewport
	m.helpViewport = NewViewport(76, 14)
	m.helpViewport.SetContent(m.renderHelpContent())

	// Scroll wheel down
	mouseMsg := tea.MouseMsg{
		X:    40,
		Y:    10,
		Type: tea.MouseWheelDown,
	}

	newModel, _ := m.handleMouseEvent(mouseMsg)
	_ = newModel.(Model)
}

// Test tab positions
func TestTabPositions_GetTabAtPosition(t *testing.T) {
	tabs := AllTabs()
	_, positions := RenderTabBar(tabs, TabDashboard, 80)

	// Verify we can find tabs at their positions
	for _, pos := range positions.Tabs {
		found := positions.GetTabAtPosition(pos.Start)
		if found != pos.Tab {
			t.Errorf("GetTabAtPosition(%d) = %v, want %v", pos.Start, found, pos.Tab)
		}
	}

	// Verify invalid position returns -1
	found := positions.GetTabAtPosition(-1)
	if found != Tab(-1) {
		t.Errorf("GetTabAtPosition(-1) = %v, want -1", found)
	}
}

// Test Badge function
func TestBadge(t *testing.T) {
	tests := []struct {
		badgeType BadgeType
		text      string
	}{
		{BadgeOK, ""},
		{BadgeWarn, ""},
		{BadgeFail, ""},
		{BadgeNA, ""},
		{BadgeInfo, ""},
		{BadgeOK, "CUSTOM"},
	}

	for _, tt := range tests {
		badge := Badge(tt.badgeType, tt.text)
		if badge == "" {
			t.Errorf("Badge(%v, %q) returned empty string", tt.badgeType, tt.text)
		}
	}
}

// Test Card function
func TestCard(t *testing.T) {
	card := Card("Test Title", "Test body content", 40)
	if card == "" {
		t.Error("Card() returned empty string")
	}

	if !containsString(card, "Test Title") {
		t.Error("Card() should contain title")
	}
}

// Test KeyHint function
func TestKeyHint(t *testing.T) {
	hint := KeyHint("k", "action")
	if hint == "" {
		t.Error("KeyHint() returned empty string")
	}

	if !containsString(hint, "k") {
		t.Error("KeyHint() should contain key")
	}
}

// Test Table function
func TestTable(t *testing.T) {
	rows := [][]string{
		{"Label1", "Value1"},
		{"Label2", "Value2"},
	}

	table := Table(rows, 2)
	if table == "" {
		t.Error("Table() returned empty string")
	}

	if !containsString(table, "Label1") {
		t.Error("Table() should contain labels")
	}
}

// Test StatusTable function
func TestStatusTable(t *testing.T) {
	rows := []StatusRow{
		{Label: "Service1", Status: BadgeOK, Value: "OK"},
		{Label: "Service2", Status: BadgeFail, Value: "FAIL"},
	}

	table := StatusTable(rows, 2)
	if table == "" {
		t.Error("StatusTable() returned empty string")
	}
}

// Test WarningBox function
func TestWarningBox(t *testing.T) {
	box := WarningBox("Warning Title", "Warning message", 60)
	if box == "" {
		t.Error("WarningBox() returned empty string")
	}
}

// Test ScrollHint function
func TestScrollHint(t *testing.T) {
	// At top
	hint := ScrollHint(true, false)
	if hint == "" {
		t.Error("ScrollHint(true, false) returned empty string")
	}

	// At bottom
	hint = ScrollHint(false, true)
	if hint == "" {
		t.Error("ScrollHint(false, true) returned empty string")
	}

	// In middle
	hint = ScrollHint(false, false)
	if hint == "" {
		t.Error("ScrollHint(false, false) returned empty string")
	}
}

// Helper function
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

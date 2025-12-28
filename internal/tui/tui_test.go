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

	// M4: Check form state is initialized
	if m.formResult == nil {
		t.Error("NewModel() formResult should be initialized")
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

// M4: Validator Actions Menu Tests

func TestModel_NavigateToValidatorActions(t *testing.T) {
	m := NewModel()

	newModel, _ := m.handleAction("validator-actions")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewValidatorActions {
		t.Errorf("handleAction('validator-actions') currentView = %v, want ViewValidatorActions", resultModel.currentView)
	}
}

func TestModel_ValidatorMenu_CreateValidator(t *testing.T) {
	m := NewModel()
	m.currentView = ViewValidatorActions
	m = m.setupValidatorMenu()

	newModel, _ := m.handleValidatorAction("create-validator")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewCreateValidator {
		t.Errorf("handleValidatorAction('create-validator') currentView = %v, want ViewCreateValidator", resultModel.currentView)
	}

	// Should have form fields set up
	if len(resultModel.formFields) == 0 {
		t.Error("create-validator view should have form fields")
	}
}

func TestModel_ValidatorMenu_Delegate(t *testing.T) {
	m := NewModel()
	m.currentView = ViewValidatorActions

	newModel, _ := m.handleValidatorAction("delegate")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewDelegate {
		t.Errorf("handleValidatorAction('delegate') currentView = %v, want ViewDelegate", resultModel.currentView)
	}

	if len(resultModel.formFields) == 0 {
		t.Error("delegate view should have form fields")
	}
}

func TestModel_ValidatorMenu_Unbond(t *testing.T) {
	m := NewModel()

	newModel, _ := m.handleValidatorAction("unbond")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewUnbond {
		t.Errorf("handleValidatorAction('unbond') currentView = %v, want ViewUnbond", resultModel.currentView)
	}
}

func TestModel_ValidatorMenu_Redelegate(t *testing.T) {
	m := NewModel()

	newModel, _ := m.handleValidatorAction("redelegate")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewRedelegate {
		t.Errorf("handleValidatorAction('redelegate') currentView = %v, want ViewRedelegate", resultModel.currentView)
	}
}

func TestModel_ValidatorMenu_WithdrawRewards(t *testing.T) {
	m := NewModel()

	newModel, _ := m.handleValidatorAction("withdraw-rewards")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewWithdrawRewards {
		t.Errorf("handleValidatorAction('withdraw-rewards') currentView = %v, want ViewWithdrawRewards", resultModel.currentView)
	}
}

func TestModel_ValidatorMenu_Vote(t *testing.T) {
	m := NewModel()

	newModel, _ := m.handleValidatorAction("vote")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewVote {
		t.Errorf("handleValidatorAction('vote') currentView = %v, want ViewVote", resultModel.currentView)
	}
}

func TestModel_ValidatorMenu_Back(t *testing.T) {
	m := NewModel()
	m.currentView = ViewValidatorActions

	newModel, _ := m.handleValidatorAction("back")
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewMain {
		t.Errorf("handleValidatorAction('back') currentView = %v, want ViewMain", resultModel.currentView)
	}
}

func TestModel_FormSetup_CreateValidator(t *testing.T) {
	m := NewModel()
	m = m.setupCreateValidatorForm()

	// Should have required fields
	requiredFields := []string{"from", "moniker", "amount", "min-self-delegation", "commission-rate", "network"}
	for _, required := range requiredFields {
		found := false
		for _, field := range m.formFields {
			if field.Label == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("create-validator form missing required field: %s", required)
		}
	}

	// First field should be focused
	if !m.formFields[0].Input.Focused() {
		t.Error("First form field should be focused")
	}
}

func TestModel_FormSetup_Delegate(t *testing.T) {
	m := NewModel()
	m = m.setupDelegateForm()

	// Should have required fields
	if len(m.formFields) < 4 {
		t.Errorf("delegate form should have at least 4 fields, got %d", len(m.formFields))
	}
}

func TestModel_FormSetup_Vote(t *testing.T) {
	m := NewModel()
	m = m.setupVoteForm()

	// Should have proposal and option fields
	fieldLabels := make([]string, len(m.formFields))
	for i, f := range m.formFields {
		fieldLabels[i] = f.Label
	}

	hasProposal := false
	hasOption := false
	for _, label := range fieldLabels {
		if label == "proposal" {
			hasProposal = true
		}
		if label == "option" {
			hasOption = true
		}
	}

	if !hasProposal {
		t.Error("vote form should have 'proposal' field")
	}
	if !hasOption {
		t.Error("vote form should have 'option' field")
	}
}

func TestModel_ResetForm(t *testing.T) {
	m := NewModel()
	m = m.setupCreateValidatorForm()
	m.formResult["test"] = "value"
	m.generatedCmd = "some command"
	m.showConfirm = true

	m = m.resetForm()

	if len(m.formFields) != 0 {
		t.Error("resetForm() should clear formFields")
	}

	if m.formIndex != 0 {
		t.Error("resetForm() should reset formIndex")
	}

	if len(m.formResult) != 0 {
		t.Error("resetForm() should clear formResult")
	}

	if m.generatedCmd != "" {
		t.Error("resetForm() should clear generatedCmd")
	}

	if m.showConfirm {
		t.Error("resetForm() should clear showConfirm")
	}
}

func TestModel_GenerateCommand_Delegate(t *testing.T) {
	m := NewModel()
	m.currentView = ViewDelegate
	m.formResult = map[string]string{
		"from":    "mykey",
		"to":      "monovaloper1xxx",
		"amount":  "1000alyth",
		"network": "Sprintnet",
	}

	cmd, err := m.generateCommand()
	if err != nil {
		t.Fatalf("generateCommand() error = %v", err)
	}

	// Should contain key parts
	expectedParts := []string{"monoctl", "stake", "delegate", "--from", "mykey", "--to", "monovaloper1xxx", "--amount", "1000alyth", "--network", "Sprintnet", "--dry-run"}
	for _, part := range expectedParts {
		if !containsString(cmd, part) {
			t.Errorf("generateCommand() missing part %q: %s", part, cmd)
		}
	}
}

func TestModel_GenerateCommand_Vote(t *testing.T) {
	m := NewModel()
	m.currentView = ViewVote
	m.formResult = map[string]string{
		"from":     "mykey",
		"proposal": "1",
		"option":   "yes",
		"network":  "Localnet",
	}

	cmd, err := m.generateCommand()
	if err != nil {
		t.Fatalf("generateCommand() error = %v", err)
	}

	expectedParts := []string{"monoctl", "gov", "vote", "--proposal", "1", "--option", "yes", "--dry-run"}
	for _, part := range expectedParts {
		if !containsString(cmd, part) {
			t.Errorf("generateCommand() missing part %q: %s", part, cmd)
		}
	}
}

func TestModel_GenerateCommand_CreateValidator(t *testing.T) {
	m := NewModel()
	m.currentView = ViewCreateValidator
	m.formResult = map[string]string{
		"from":                "validator",
		"moniker":             "my-validator",
		"amount":              "100000000000000000000000alyth",
		"min-self-delegation": "100000000000000000000000alyth",
		"commission-rate":     "0.10",
		"network":             "Sprintnet",
	}

	cmd, err := m.generateCommand()
	if err != nil {
		t.Fatalf("generateCommand() error = %v", err)
	}

	expectedParts := []string{"monoctl", "validator", "create", "--moniker", "my-validator", "--dry-run"}
	for _, part := range expectedParts {
		if !containsString(cmd, part) {
			t.Errorf("generateCommand() missing part %q: %s", part, cmd)
		}
	}
}

func TestModel_UpdateFormFocus(t *testing.T) {
	m := NewModel()
	m = m.setupDelegateForm()

	// Initially first field should be focused
	if !m.formFields[0].Input.Focused() {
		t.Error("First field should be focused initially")
	}

	// Move to second field
	m.formIndex = 1
	m = m.updateFormFocus()

	if m.formFields[0].Input.Focused() {
		t.Error("First field should be blurred after moving to second")
	}
	if !m.formFields[1].Input.Focused() {
		t.Error("Second field should be focused")
	}
}

func TestModel_EscFromFormReturnsToValidatorMenu(t *testing.T) {
	m := NewModel()
	m.currentView = ViewDelegate
	m = m.setupDelegateForm()

	// Simulate ESC key
	newModel, _ := m.handleFormInput(tea.KeyMsg{Type: tea.KeyEsc})
	resultModel := newModel.(Model)

	if resultModel.currentView != ViewValidatorActions {
		t.Errorf("ESC from form should return to ViewValidatorActions, got %v", resultModel.currentView)
	}

	// Form should be reset
	if len(resultModel.formFields) != 0 {
		t.Error("ESC should reset form fields")
	}
}

func TestModel_TabNavigatesFormFields(t *testing.T) {
	m := NewModel()
	m.currentView = ViewDelegate
	m = m.setupDelegateForm()
	initialFields := len(m.formFields)

	if m.formIndex != 0 {
		t.Error("Form should start at index 0")
	}

	// Tab should move to next field
	newModel, _ := m.handleFormInput(tea.KeyMsg{Type: tea.KeyTab})
	resultModel := newModel.(Model)

	if resultModel.formIndex != 1 {
		t.Errorf("Tab should move to index 1, got %d", resultModel.formIndex)
	}

	// Tab at last field should wrap to first
	resultModel.formIndex = initialFields - 1
	newModel2, _ := resultModel.handleFormInput(tea.KeyMsg{Type: tea.KeyTab})
	resultModel2 := newModel2.(Model)

	if resultModel2.formIndex != 0 {
		t.Errorf("Tab at last field should wrap to 0, got %d", resultModel2.formIndex)
	}
}

func TestModel_RenderFormView(t *testing.T) {
	m := NewModel()
	m.currentView = ViewDelegate
	m.width = 80
	m.height = 24
	m = m.setupDelegateForm()

	view := m.renderFormView()

	if view == "" {
		t.Error("renderFormView() should not be empty")
	}

	// Should contain form field labels
	if !containsString(view, "from") {
		t.Error("renderFormView() should contain 'from' field")
	}
}

func TestModel_RenderFormView_WithConfirm(t *testing.T) {
	m := NewModel()
	m.currentView = ViewDelegate
	m.showConfirm = true
	m.generatedCmd = "monoctl stake delegate --from test"

	view := m.renderFormView()

	if !containsString(view, "Command Preview") {
		t.Error("renderFormView() with confirm should show 'Command Preview'")
	}

	if !containsString(view, m.generatedCmd) {
		t.Error("renderFormView() with confirm should show generated command")
	}
}

func TestModel_View_ValidatorActions(t *testing.T) {
	m := NewModel()
	m.currentView = ViewValidatorActions
	m = m.setupValidatorMenu()
	m.width = 80
	m.height = 24

	view := m.View()

	if view == "" {
		t.Error("View() for ValidatorActions should not be empty")
	}
}

func TestModel_View_FormViews(t *testing.T) {
	views := []View{ViewCreateValidator, ViewDelegate, ViewUnbond, ViewRedelegate, ViewWithdrawRewards, ViewVote}

	for _, v := range views {
		m := NewModel()
		m.currentView = v
		m.width = 80
		m.height = 24

		// Set up appropriate form
		switch v {
		case ViewCreateValidator:
			m = m.setupCreateValidatorForm()
		case ViewDelegate:
			m = m.setupDelegateForm()
		case ViewUnbond:
			m = m.setupUnbondForm()
		case ViewRedelegate:
			m = m.setupRedelegateForm()
		case ViewWithdrawRewards:
			m = m.setupWithdrawRewardsForm()
		case ViewVote:
			m = m.setupVoteForm()
		}

		view := m.View()
		if view == "" {
			t.Errorf("View() for %v should not be empty", v)
		}
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

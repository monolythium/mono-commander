package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Golden tests verify command generation matches expected outputs

func TestGolden_DelegateCommand(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkSprintnet,
		Home:    "/home/user/.monod",
		From:    "mykey",
		Fees:    "10000alyth",
	}

	params := DelegateParams{
		ValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		Amount:        "1000000000000000000alyth",
	}

	cmd, err := BuildDelegateTx(opts, params)
	if err != nil {
		t.Fatalf("BuildDelegateTx() error = %v", err)
	}

	// Load golden file
	golden := loadGolden(t, "delegate_cmd.txt")
	got := cmd.String()

	if got != golden {
		t.Errorf("BuildDelegateTx() command mismatch:\ngot:\n%s\n\nwant:\n%s", got, golden)
	}
}

func TestGolden_UnbondCommand(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkLocalnet,
		From:    "mykey",
	}

	params := UnbondParams{
		ValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		Amount:        "1000000000000000000alyth",
	}

	cmd, err := BuildUnbondTx(opts, params)
	if err != nil {
		t.Fatalf("BuildUnbondTx() error = %v", err)
	}

	golden := loadGolden(t, "unbond_cmd.txt")
	got := cmd.String()

	if got != golden {
		t.Errorf("BuildUnbondTx() command mismatch:\ngot:\n%s\n\nwant:\n%s", got, golden)
	}
}

func TestGolden_RedelegateCommand(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkLocalnet,
		From:    "mykey",
	}

	params := RedelegateParams{
		SrcValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		DstValidatorAddr: "monovaloper1wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww5nfrmp",
		Amount:           "1000000000000000000alyth",
	}

	cmd, err := BuildRedelegateTx(opts, params)
	if err != nil {
		t.Fatalf("BuildRedelegateTx() error = %v", err)
	}

	golden := loadGolden(t, "redelegate_cmd.txt")
	got := cmd.String()

	if got != golden {
		t.Errorf("BuildRedelegateTx() command mismatch:\ngot:\n%s\n\nwant:\n%s", got, golden)
	}
}

func TestGolden_VoteCommand(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkLocalnet,
		From:    "mykey",
	}

	params := VoteParams{
		ProposalID: "1",
		Option:     VoteYes,
	}

	cmd, err := BuildVoteTx(opts, params)
	if err != nil {
		t.Fatalf("BuildVoteTx() error = %v", err)
	}

	golden := loadGolden(t, "vote_cmd.txt")
	got := cmd.String()

	if got != golden {
		t.Errorf("BuildVoteTx() command mismatch:\ngot:\n%s\n\nwant:\n%s", got, golden)
	}
}

// loadGolden loads a golden file from testdata/golden/
func loadGolden(t *testing.T, filename string) string {
	t.Helper()

	// Find the testdata directory
	path := filepath.Join("..", "..", "testdata", "golden", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load golden file %s: %v", filename, err)
	}

	return strings.TrimSpace(string(data))
}

// TestCommandGeneration_Deterministic verifies commands are generated consistently
func TestCommandGeneration_Deterministic(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkSprintnet,
		Home:    "/home/user/.monod",
		From:    "testkey",
		Fees:    "10000alyth",
	}

	params := DelegateParams{
		ValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		Amount:        "1000000000000000000alyth",
	}

	// Generate command multiple times
	var commands []string
	for i := 0; i < 5; i++ {
		cmd, err := BuildDelegateTx(opts, params)
		if err != nil {
			t.Fatalf("BuildDelegateTx() iteration %d error = %v", i, err)
		}
		commands = append(commands, cmd.String())
	}

	// All commands should be identical
	for i := 1; i < len(commands); i++ {
		if commands[i] != commands[0] {
			t.Errorf("Command generation not deterministic:\n  first:   %s\n  iter %d: %s", commands[0], i, commands[i])
		}
	}
}

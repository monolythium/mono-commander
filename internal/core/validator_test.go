package core

import (
	"context"
	"testing"
)

func TestDelegateAction_DryRun(t *testing.T) {
	opts := ValidatorActionOptions{
		Network: NetworkLocalnet,
		Home:    "/tmp/test",
		From:    "testkey",
		DryRun:  true,
		Execute: false,
	}

	params := DelegateParams{
		ValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		Amount:        "1000000000000000000alyth",
	}

	ctx := context.Background()
	result, err := DelegateAction(ctx, opts, params)
	if err != nil {
		t.Fatalf("DelegateAction() error = %v", err)
	}

	if result.Executed {
		t.Error("DelegateAction() should not execute in dry-run mode")
	}

	if result.Command == nil {
		t.Error("DelegateAction() should return a command")
	}

	// Check steps
	if len(result.Steps) < 2 {
		t.Errorf("DelegateAction() should have at least 2 steps, got %d", len(result.Steps))
	}

	// First step should be network validation
	if result.Steps[0].Name != "Validate network" || result.Steps[0].Status != "success" {
		t.Errorf("DelegateAction() first step = %+v", result.Steps[0])
	}
}

func TestDelegateAction_InvalidNetwork(t *testing.T) {
	opts := ValidatorActionOptions{
		Network: NetworkName("invalid"),
		Home:    "/tmp/test",
		From:    "testkey",
		DryRun:  true,
	}

	params := DelegateParams{
		ValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		Amount:        "1000000000000000000alyth",
	}

	ctx := context.Background()
	result, err := DelegateAction(ctx, opts, params)
	if err == nil {
		t.Error("DelegateAction() should error on invalid network")
	}

	if result != nil && len(result.Steps) > 0 {
		if result.Steps[0].Status != "failed" {
			t.Error("DelegateAction() network validation step should fail")
		}
	}
}

func TestUnbondAction_DryRun(t *testing.T) {
	opts := ValidatorActionOptions{
		Network: NetworkSprintnet,
		Home:    "/tmp/test",
		From:    "testkey",
		DryRun:  true,
	}

	params := UnbondParams{
		ValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		Amount:        "1000000000000000000alyth",
	}

	ctx := context.Background()
	result, err := UnbondAction(ctx, opts, params)
	if err != nil {
		t.Fatalf("UnbondAction() error = %v", err)
	}

	if result.Executed {
		t.Error("UnbondAction() should not execute in dry-run mode")
	}

	// Should have warnings about unbonding period
	if len(result.Warnings) == 0 {
		t.Error("UnbondAction() should have warnings about unbonding period")
	}
}

func TestRedelegateAction_DryRun(t *testing.T) {
	opts := ValidatorActionOptions{
		Network: NetworkTestnet,
		Home:    "/tmp/test",
		From:    "testkey",
		DryRun:  true,
	}

	params := RedelegateParams{
		SrcValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		DstValidatorAddr: "monovaloper1wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww5nfrmp",
		Amount:           "1000000000000000000alyth",
	}

	ctx := context.Background()
	result, err := RedelegateAction(ctx, opts, params)
	if err != nil {
		t.Fatalf("RedelegateAction() error = %v", err)
	}

	if result.Executed {
		t.Error("RedelegateAction() should not execute in dry-run mode")
	}

	if result.Action != TxActionRedelegate {
		t.Errorf("RedelegateAction() action = %v, want %v", result.Action, TxActionRedelegate)
	}
}

func TestWithdrawRewardsAction_DryRun(t *testing.T) {
	opts := ValidatorActionOptions{
		Network: NetworkMainnet,
		Home:    "/tmp/test",
		From:    "testkey",
		DryRun:  true,
	}

	params := WithdrawRewardsParams{
		ValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
		Commission:    false,
	}

	ctx := context.Background()
	result, err := WithdrawRewardsAction(ctx, opts, params)
	if err != nil {
		t.Fatalf("WithdrawRewardsAction() error = %v", err)
	}

	if result.Executed {
		t.Error("WithdrawRewardsAction() should not execute in dry-run mode")
	}

	if result.Action != TxActionWithdrawRewards {
		t.Errorf("WithdrawRewardsAction() action = %v, want %v", result.Action, TxActionWithdrawRewards)
	}
}

func TestVoteAction_DryRun(t *testing.T) {
	opts := ValidatorActionOptions{
		Network: NetworkLocalnet,
		Home:    "/tmp/test",
		From:    "testkey",
		DryRun:  true,
	}

	params := VoteParams{
		ProposalID: "1",
		Option:     VoteYes,
	}

	ctx := context.Background()
	result, err := VoteAction(ctx, opts, params)
	if err != nil {
		t.Fatalf("VoteAction() error = %v", err)
	}

	if result.Executed {
		t.Error("VoteAction() should not execute in dry-run mode")
	}

	if result.Action != TxActionVote {
		t.Errorf("VoteAction() action = %v, want %v", result.Action, TxActionVote)
	}
}

func TestCreateValidatorAction_DryRun(t *testing.T) {
	opts := ValidatorActionOptions{
		Network: NetworkSprintnet,
		Home:    "/tmp/test",
		From:    "validator",
		DryRun:  true,
	}

	params := CreateValidatorParams{
		Moniker:             "test-validator",
		CommissionRate:      "0.10",
		CommissionMaxRate:   "0.20",
		CommissionMaxChange: "0.01",
		MinSelfDelegation:   "100000000000000000000000alyth",
		Amount:              "100000000000000000000000alyth",
	}

	ctx := context.Background()
	result, err := CreateValidatorAction(ctx, opts, params)
	if err != nil {
		t.Fatalf("CreateValidatorAction() error = %v", err)
	}

	if result.Executed {
		t.Error("CreateValidatorAction() should not execute in dry-run mode")
	}

	if result.Action != TxActionCreateValidator {
		t.Errorf("CreateValidatorAction() action = %v, want %v", result.Action, TxActionCreateValidator)
	}

	// Should have warnings about burn
	if len(result.Warnings) == 0 {
		t.Error("CreateValidatorAction() should have warnings about 100k LYTH burn")
	}
}

func TestValidatorActionOptions_ToTxBuilderOptions(t *testing.T) {
	opts := ValidatorActionOptions{
		Network:   NetworkSprintnet,
		Home:      "/home/user/.monod",
		From:      "mykey",
		Fees:      "10000alyth",
		GasPrices: "",
		Gas:       "auto",
		Node:      "http://localhost:26657",
		ChainID:   "",
		DryRun:    true,
		Execute:   false,
	}

	txOpts := opts.toTxBuilderOptions()

	if txOpts.Network != opts.Network {
		t.Errorf("toTxBuilderOptions() Network = %v, want %v", txOpts.Network, opts.Network)
	}

	if txOpts.Home != opts.Home {
		t.Errorf("toTxBuilderOptions() Home = %v, want %v", txOpts.Home, opts.Home)
	}

	if txOpts.From != opts.From {
		t.Errorf("toTxBuilderOptions() From = %v, want %v", txOpts.From, opts.From)
	}

	if txOpts.Broadcast {
		t.Error("toTxBuilderOptions() Broadcast should be false when DryRun=true")
	}

	if !txOpts.DryRun {
		t.Error("toTxBuilderOptions() DryRun should be true")
	}
}

func TestValidatorActionOptions_ExecuteOverridesDryRun(t *testing.T) {
	opts := ValidatorActionOptions{
		Network: NetworkLocalnet,
		Home:    "/tmp/test",
		From:    "mykey",
		DryRun:  true,
		Execute: true, // Execute should override DryRun
	}

	txOpts := opts.toTxBuilderOptions()

	// When Execute is true, DryRun should be false
	if txOpts.DryRun {
		t.Error("toTxBuilderOptions() Execute should override DryRun")
	}

	if !txOpts.Broadcast {
		t.Error("toTxBuilderOptions() Broadcast should be true when Execute=true")
	}
}

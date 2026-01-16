// Package core provides the core logic for mono-commander.
package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCombineMessages(t *testing.T) {
	// Sample unsigned transaction JSONs (simplified)
	tx1JSON := []byte(`{
		"body": {
			"messages": [
				{"@type": "/cosmos.staking.v1beta1.MsgCreateValidator", "moniker": "test"}
			],
			"memo": "",
			"timeout_height": "0",
			"extension_options": [],
			"non_critical_extension_options": []
		},
		"auth_info": {
			"signer_infos": [],
			"fee": {
				"amount": [],
				"gas_limit": "200000",
				"payer": "",
				"granter": ""
			}
		},
		"signatures": []
	}`)

	tx2JSON := []byte(`{
		"body": {
			"messages": [
				{"@type": "/cosmos.bank.v1beta1.MsgBurn", "amount": "100000000000000000000000alyth"}
			],
			"memo": "",
			"timeout_height": "0",
			"extension_options": [],
			"non_critical_extension_options": []
		},
		"auth_info": {
			"signer_infos": [],
			"fee": {
				"amount": [],
				"gas_limit": "100000",
				"payer": "",
				"granter": ""
			}
		},
		"signatures": []
	}`)

	executor := &MultiMsgExecutor{
		GasAdjustment: 1.5,
	}

	// Test combining two transactions
	combined, err := executor.CombineMessages(tx1JSON, tx2JSON)
	if err != nil {
		t.Fatalf("CombineMessages failed: %v", err)
	}

	// Parse combined result
	var combinedTx TxJSON
	if err := json.Unmarshal(combined, &combinedTx); err != nil {
		t.Fatalf("Failed to parse combined tx: %v", err)
	}

	// Verify two messages
	if len(combinedTx.Body.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(combinedTx.Body.Messages))
	}

	// Verify gas is adjusted (200000 + 100000) * 1.5 = 450000
	expectedGas := "450000"
	if combinedTx.AuthInfo.Fee.GasLimit != expectedGas {
		t.Errorf("Expected gas %s, got %s", expectedGas, combinedTx.AuthInfo.Fee.GasLimit)
	}
}

func TestCombineMessages_SingleTx(t *testing.T) {
	singleTxJSON := []byte(`{
		"body": {
			"messages": [
				{"@type": "/cosmos.bank.v1beta1.MsgSend"}
			],
			"memo": "",
			"timeout_height": "0",
			"extension_options": [],
			"non_critical_extension_options": []
		},
		"auth_info": {
			"signer_infos": [],
			"fee": {
				"amount": [],
				"gas_limit": "100000",
				"payer": "",
				"granter": ""
			}
		},
		"signatures": []
	}`)

	executor := &MultiMsgExecutor{
		GasAdjustment: 1.5,
	}

	// Single transaction should be returned as-is
	result, err := executor.CombineMessages(singleTxJSON)
	if err != nil {
		t.Fatalf("CombineMessages failed: %v", err)
	}

	// Should be the same (or similar) to input
	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestCombineMessages_Empty(t *testing.T) {
	executor := &MultiMsgExecutor{}

	_, err := executor.CombineMessages()
	if err == nil {
		t.Error("Expected error for empty transactions")
	}
}

func TestCombineMessages_InvalidJSON(t *testing.T) {
	executor := &MultiMsgExecutor{}

	// Need at least 2 txs to trigger parsing (single tx is returned as-is)
	validJSON := []byte(`{
		"body": {"messages": [], "memo": "", "timeout_height": "0", "extension_options": [], "non_critical_extension_options": []},
		"auth_info": {"signer_infos": [], "fee": {"amount": [], "gas_limit": "100000", "payer": "", "granter": ""}},
		"signatures": []
	}`)

	_, err := executor.CombineMessages([]byte("not json"), validJSON)
	if err == nil {
		t.Error("Expected error for invalid JSON in first position")
	}

	_, err = executor.CombineMessages(validJSON, []byte("also not json"))
	if err == nil {
		t.Error("Expected error for invalid JSON in second position")
	}
}

func TestBuildCreateValidatorTx_HasMultiMsg(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkSprintnet,
		From:    "testkey",
		Home:    "/home/test/.monod",
	}

	params := CreateValidatorParams{
		Moniker:             "test-validator",
		CommissionRate:      "0.10",
		CommissionMaxRate:   "0.20",
		CommissionMaxChange: "0.01",
		MinSelfDelegation:   LYTHToAlyth(100000),
		Amount:              LYTHToAlyth(100000),
	}

	cmd, err := BuildCreateValidatorTx(opts, params)
	if err != nil {
		t.Fatalf("BuildCreateValidatorTx failed: %v", err)
	}

	// Must require multi-message
	if !cmd.RequiresMultiMsg {
		t.Error("Expected RequiresMultiMsg to be true")
	}

	// Must have exactly 2 sub-commands (create-validator and burn)
	if len(cmd.MultiMsgCommands) != 2 {
		t.Errorf("Expected 2 MultiMsgCommands, got %d", len(cmd.MultiMsgCommands))
	}

	// Verify first command is create-validator
	if cmd.MultiMsgCommands[0].Action != TxActionCreateValidator {
		t.Errorf("Expected first command to be create-validator, got %s", cmd.MultiMsgCommands[0].Action)
	}

	// Verify second command is burn
	if cmd.MultiMsgCommands[1].Action != TxAction("burn") {
		t.Errorf("Expected second command to be burn, got %s", cmd.MultiMsgCommands[1].Action)
	}

	// Verify sub-commands have --generate-only (important for multi-msg flow)
	for i, subCmd := range cmd.MultiMsgCommands {
		hasGenerateOnly := false
		for _, arg := range subCmd.Args {
			if arg == "--generate-only" {
				hasGenerateOnly = true
				break
			}
		}
		if !hasGenerateOnly {
			t.Errorf("MultiMsgCommand[%d] missing --generate-only flag", i)
		}
	}
}

func TestGetMultiMsgPreviewCommands(t *testing.T) {
	cmd := &TxCommand{
		RequiresMultiMsg: true,
		MultiMsgCommands: []*TxCommand{
			{
				Action:      TxActionCreateValidator,
				Binary:      "monod",
				Args:        []string{"tx", "staking", "create-validator"},
				Description: "Create validator",
			},
			{
				Action:      TxAction("burn"),
				Binary:      "monod",
				Args:        []string{"tx", "bank", "burn"},
				Description: "Burn tokens",
			},
		},
	}

	preview := GetMultiMsgPreviewCommands(cmd)

	if len(preview) == 0 {
		t.Error("Expected non-empty preview")
	}

	// Should contain "Multi-message"
	found := false
	for _, line := range preview {
		if line == "# Multi-message transaction preview:" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Preview should mention multi-message")
	}
}

func TestFormatMultiMsgDryRun(t *testing.T) {
	cmd := &TxCommand{
		Description:      "Create validator test",
		RequiresMultiMsg: true,
		WarningMessages: []string{
			"This requires burning 100k LYTH",
		},
		MultiMsgCommands: []*TxCommand{
			{
				Description: "Create validator message",
				Binary:      "monod",
				Args:        []string{"tx", "staking", "create-validator"},
			},
			{
				Description: "Burn 100k LYTH",
				Binary:      "monod",
				Args:        []string{"tx", "bank", "burn"},
			},
		},
	}

	output := FormatMultiMsgDryRun(cmd)

	// Should contain description
	if output == "" {
		t.Error("Expected non-empty output")
	}

	// Should mention multi-message
	if !strings.Contains(output, "Multi-Message") {
		t.Error("Output should mention Multi-Message")
	}

	// Should mention both messages
	if !strings.Contains(output, "Create validator") {
		t.Error("Output should mention Create validator")
	}
	if !strings.Contains(output, "Burn") {
		t.Error("Output should mention Burn")
	}
}

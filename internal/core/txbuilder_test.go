package core

import (
	"math/big"
	"strings"
	"testing"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid address", "mono1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp", false},
		{"empty address", "", true},
		{"too short", "mono1abc", true},
		{"wrong prefix", "cosmos1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp", true},
		{"wrong length", "mono1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq", true},
		{"uppercase not allowed", "mono1QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ5nfrmp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddress(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateValoperAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid valoper", "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp", false},
		{"empty address", "", true},
		{"wrong prefix", "mono1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp", true},
		{"too short", "monovaloper1abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateValoperAddress(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateValoperAddress(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAmount(t *testing.T) {
	tests := []struct {
		name    string
		amount  string
		wantErr bool
	}{
		{"valid amount", "1000000000000000000alyth", false},
		{"large amount", "100000000000000000000000alyth", false},
		{"empty amount", "", true},
		{"wrong denom", "1000ulyth", true},
		{"no denom", "1000", true},
		{"zero amount", "0alyth", true},
		{"negative amount", "-1000alyth", true},
		{"non-numeric", "abcalyth", true},
		{"only denom", "alyth", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAmount(tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAmount(%q) error = %v, wantErr %v", tt.amount, err, tt.wantErr)
			}
		})
	}
}

func TestMinSelfDelegationAlyth(t *testing.T) {
	// 100,000 LYTH = 100000 * 10^18 alyth
	expected := "100000000000000000000000"

	if MinSelfDelegationAlyth.String() != expected {
		t.Errorf("MinSelfDelegationAlyth = %s, want %s", MinSelfDelegationAlyth.String(), expected)
	}
}

func TestValidatorBurnAlyth(t *testing.T) {
	// 100,000 LYTH = 100000 * 10^18 alyth
	expected := "100000000000000000000000"

	if ValidatorBurnAlyth.String() != expected {
		t.Errorf("ValidatorBurnAlyth = %s, want %s", ValidatorBurnAlyth.String(), expected)
	}
}

func TestValidateMinSelfDelegation(t *testing.T) {
	tests := []struct {
		name    string
		amount  string
		wantErr bool
	}{
		{"exactly 100k LYTH", "100000000000000000000000alyth", false},
		{"more than 100k LYTH", "200000000000000000000000alyth", false},
		{"less than 100k LYTH", "99999000000000000000000alyth", true},
		{"1 LYTH", "1000000000000000000alyth", true},
		{"invalid format", "100000alyth", true}, // This is only 100000 alyth, not LYTH
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMinSelfDelegation(tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMinSelfDelegation(%q) error = %v, wantErr %v", tt.amount, err, tt.wantErr)
			}
		})
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name      string
		amount    string
		wantValue string
		wantErr   bool
	}{
		{"1 LYTH in alyth", "1000000000000000000alyth", "1000000000000000000", false},
		{"100k LYTH in alyth", "100000000000000000000000alyth", "100000000000000000000000", false},
		{"invalid", "invalidalyth", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAmount(tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAmount(%q) error = %v, wantErr %v", tt.amount, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.wantValue {
				t.Errorf("ParseAmount(%q) = %s, want %s", tt.amount, got.String(), tt.wantValue)
			}
		})
	}
}

func TestFormatLYTH(t *testing.T) {
	tests := []struct {
		name  string
		alyth *big.Int
		want  string
	}{
		{"nil", nil, "0 LYTH"},
		{"zero", big.NewInt(0), "0 LYTH"},
		{"1 LYTH", new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil), "1 LYTH"},
		{"100 LYTH", new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)), "100 LYTH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLYTH(tt.alyth)
			if got != tt.want {
				t.Errorf("FormatLYTH() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestLYTHToAlyth(t *testing.T) {
	tests := []struct {
		lyth int64
		want string
	}{
		{1, "1000000000000000000alyth"},
		{100, "100000000000000000000alyth"},
		{100000, "100000000000000000000000alyth"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := LYTHToAlyth(tt.lyth)
			if got != tt.want {
				t.Errorf("LYTHToAlyth(%d) = %s, want %s", tt.lyth, got, tt.want)
			}
		})
	}
}

func TestValidateVoteOption(t *testing.T) {
	tests := []struct {
		input   string
		want    VoteOption
		wantErr bool
	}{
		{"yes", VoteYes, false},
		{"YES", VoteYes, false},
		{"1", VoteYes, false},
		{"no", VoteNo, false},
		{"2", VoteNo, false},
		{"abstain", VoteAbstain, false},
		{"3", VoteAbstain, false},
		{"no_with_veto", VoteNoWithVeto, false},
		{"nowithveto", VoteNoWithVeto, false},
		{"4", VoteNoWithVeto, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateVoteOption(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVoteOption(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateVoteOption(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateCommissionRate(t *testing.T) {
	tests := []struct {
		rate    string
		wantErr bool
	}{
		{"0.10", false},
		{"0.0", false},
		{"1.0", false},
		{"0.05", false},
		{"1.5", true},  // > 1.0
		{"-0.1", true}, // negative
		{"abc", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.rate, func(t *testing.T) {
			err := ValidateCommissionRate(tt.rate)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommissionRate(%q) error = %v, wantErr %v", tt.rate, err, tt.wantErr)
			}
		})
	}
}

func TestBuildDelegateTx(t *testing.T) {
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

	if cmd.Action != TxActionDelegate {
		t.Errorf("BuildDelegateTx() action = %v, want %v", cmd.Action, TxActionDelegate)
	}

	cmdStr := cmd.String()
	if !strings.Contains(cmdStr, "tx staking delegate") {
		t.Errorf("BuildDelegateTx() command missing 'tx staking delegate': %s", cmdStr)
	}
	if !strings.Contains(cmdStr, params.ValidatorAddr) {
		t.Errorf("BuildDelegateTx() command missing validator address: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, params.Amount) {
		t.Errorf("BuildDelegateTx() command missing amount: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "--chain-id mono-sprint-1") {
		t.Errorf("BuildDelegateTx() command missing chain-id: %s", cmdStr)
	}
}

func TestBuildUnbondTx(t *testing.T) {
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

	if cmd.Action != TxActionUnbond {
		t.Errorf("BuildUnbondTx() action = %v, want %v", cmd.Action, TxActionUnbond)
	}

	// Should have warning about unbonding period
	if len(cmd.WarningMessages) == 0 {
		t.Error("BuildUnbondTx() should have warning about unbonding period")
	}
}

func TestBuildRedelegateTx(t *testing.T) {
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

	if cmd.Action != TxActionRedelegate {
		t.Errorf("BuildRedelegateTx() action = %v, want %v", cmd.Action, TxActionRedelegate)
	}

	cmdStr := cmd.String()
	if !strings.Contains(cmdStr, "tx staking redelegate") {
		t.Errorf("BuildRedelegateTx() command missing 'tx staking redelegate': %s", cmdStr)
	}
}

func TestBuildWithdrawRewardsTx(t *testing.T) {
	tests := []struct {
		name        string
		params      WithdrawRewardsParams
		wantCommand string
	}{
		{
			name:        "withdraw all",
			params:      WithdrawRewardsParams{},
			wantCommand: "withdraw-all-rewards",
		},
		{
			name: "withdraw from specific validator",
			params: WithdrawRewardsParams{
				ValidatorAddr: "monovaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq5nfrmp",
			},
			wantCommand: "withdraw-rewards",
		},
		{
			name: "withdraw with commission",
			params: WithdrawRewardsParams{
				Commission: true,
			},
			wantCommand: "--commission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TxBuilderOptions{
				Network: NetworkLocalnet,
				From:    "mykey",
			}

			cmd, err := BuildWithdrawRewardsTx(opts, tt.params)
			if err != nil {
				t.Fatalf("BuildWithdrawRewardsTx() error = %v", err)
			}

			if !strings.Contains(cmd.String(), tt.wantCommand) {
				t.Errorf("BuildWithdrawRewardsTx() = %s, want to contain %s", cmd.String(), tt.wantCommand)
			}
		})
	}
}

func TestBuildVoteTx(t *testing.T) {
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

	if cmd.Action != TxActionVote {
		t.Errorf("BuildVoteTx() action = %v, want %v", cmd.Action, TxActionVote)
	}

	cmdStr := cmd.String()
	if !strings.Contains(cmdStr, "tx gov vote") {
		t.Errorf("BuildVoteTx() command missing 'tx gov vote': %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "1") {
		t.Errorf("BuildVoteTx() command missing proposal ID: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "yes") {
		t.Errorf("BuildVoteTx() command missing vote option: %s", cmdStr)
	}
}

func TestBuildVoteTx_InvalidProposal(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkLocalnet,
		From:    "mykey",
	}

	tests := []struct {
		proposalID string
	}{
		{""},
		{"abc"},
		{"-1"},
		{"0"},
	}

	for _, tt := range tests {
		t.Run(tt.proposalID, func(t *testing.T) {
			params := VoteParams{
				ProposalID: tt.proposalID,
				Option:     VoteYes,
			}

			_, err := BuildVoteTx(opts, params)
			if err == nil {
				t.Error("BuildVoteTx() expected error for invalid proposal ID")
			}
		})
	}
}

func TestBuildCreateValidatorTx(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkSprintnet,
		From:    "validator",
		Home:    "/home/user/.monod",
	}

	params := CreateValidatorParams{
		Moniker:             "my-validator",
		CommissionRate:      "0.10",
		CommissionMaxRate:   "0.20",
		CommissionMaxChange: "0.01",
		MinSelfDelegation:   "100000000000000000000000alyth",
		Amount:              "100000000000000000000000alyth",
	}

	cmd, err := BuildCreateValidatorTx(opts, params)
	if err != nil {
		t.Fatalf("BuildCreateValidatorTx() error = %v", err)
	}

	if cmd.Action != TxActionCreateValidator {
		t.Errorf("BuildCreateValidatorTx() action = %v, want %v", cmd.Action, TxActionCreateValidator)
	}

	// Should require multi-message tx (MsgCreateValidator + MsgBurn)
	if !cmd.RequiresMultiMsg {
		t.Error("BuildCreateValidatorTx() should require multi-message tx")
	}

	// Should have 2 sub-commands
	if len(cmd.MultiMsgCommands) != 2 {
		t.Errorf("BuildCreateValidatorTx() has %d sub-commands, want 2", len(cmd.MultiMsgCommands))
	}

	// Should have warnings about burn
	if len(cmd.WarningMessages) == 0 {
		t.Error("BuildCreateValidatorTx() should have warnings about burn")
	}
}

func TestBuildCreateValidatorTx_InvalidAmount(t *testing.T) {
	opts := TxBuilderOptions{
		Network: NetworkSprintnet,
		From:    "validator",
	}

	params := CreateValidatorParams{
		Moniker:             "my-validator",
		CommissionRate:      "0.10",
		CommissionMaxRate:   "0.20",
		CommissionMaxChange: "0.01",
		MinSelfDelegation:   "100000000000000000000000alyth",
		Amount:              "1000alyth", // Too low - less than 100k LYTH
	}

	_, err := BuildCreateValidatorTx(opts, params)
	if err == nil {
		t.Error("BuildCreateValidatorTx() expected error for amount below minimum")
	}
}

func TestTxCommand_String(t *testing.T) {
	cmd := &TxCommand{
		Binary: "monod",
		Args:   []string{"tx", "staking", "delegate", "monovaloper1...", "1000alyth"},
	}

	expected := "monod tx staking delegate monovaloper1... 1000alyth"
	if cmd.String() != expected {
		t.Errorf("TxCommand.String() = %s, want %s", cmd.String(), expected)
	}
}

func TestTxCommand_String_DefaultBinary(t *testing.T) {
	cmd := &TxCommand{
		Args: []string{"tx", "staking", "delegate"},
	}

	if !strings.HasPrefix(cmd.String(), "monod ") {
		t.Errorf("TxCommand.String() should default to 'monod' binary: %s", cmd.String())
	}
}

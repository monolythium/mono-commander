// Package core provides the core logic for mono-commander.
package core

import (
	"context"
	"fmt"
	"log/slog"

	oshelpers "github.com/monolythium/mono-commander/internal/os"
)

// ValidatorActionOptions contains common options for validator actions
type ValidatorActionOptions struct {
	Network        NetworkName
	Home           string
	From           string // key name or address
	Fees           string // amount in alyth
	GasPrices      string // price in alyth
	Gas            string // gas limit or "auto"
	Node           string // RPC node URL
	ChainID        string // chain-id override
	KeyringBackend string // keyring backend (os, file, test, memory)
	DryRun         bool   // default true: only show command
	Execute        bool   // if true and DryRun false, execute the command
	Logger         *slog.Logger
}

// ValidatorActionResult contains the result of a validator action
type ValidatorActionResult struct {
	Action      TxAction
	Command     *TxCommand
	Executed    bool
	Success     bool
	TxHash      string
	Height      int64
	Error       error
	Steps       []ActionStep
	Warnings    []string
	Description string
}

// ActionStep represents a step in the action process
type ActionStep struct {
	Name    string
	Status  string // "pending", "success", "failed", "skipped"
	Message string
}

// toTxBuilderOptions converts ValidatorActionOptions to TxBuilderOptions
func (o ValidatorActionOptions) toTxBuilderOptions() TxBuilderOptions {
	// Execute flag overrides DryRun - if Execute is true, we broadcast
	effectiveDryRun := o.DryRun && !o.Execute
	shouldBroadcast := o.Execute

	return TxBuilderOptions{
		Network:        o.Network,
		Home:           o.Home,
		From:           o.From,
		Fees:           o.Fees,
		GasPrices:      o.GasPrices,
		Gas:            o.Gas,
		Node:           o.Node,
		ChainID:        o.ChainID,
		KeyringBackend: o.KeyringBackend,
		Broadcast:      shouldBroadcast,
		DryRun:         effectiveDryRun,
	}
}

// CreateValidatorAction executes or previews a create-validator transaction
func CreateValidatorAction(ctx context.Context, opts ValidatorActionOptions, params CreateValidatorParams) (*ValidatorActionResult, error) {
	result := &ValidatorActionResult{
		Action: TxActionCreateValidator,
		Steps:  make([]ActionStep, 0),
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Step 1: Validate network
	result.Steps = append(result.Steps, ActionStep{Name: "Validate network", Status: "pending"})
	network, err := GetNetwork(opts.Network)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("chain-id: %s", network.ChainID)

	// Step 2: Build transaction command
	result.Steps = append(result.Steps, ActionStep{Name: "Build transaction", Status: "pending"})
	txOpts := opts.toTxBuilderOptions()
	cmd, err := BuildCreateValidatorTx(txOpts, params)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Command = cmd
	result.Description = cmd.Description
	result.Warnings = cmd.WarningMessages
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 3: Check if should execute
	if opts.DryRun || !opts.Execute {
		result.Steps = append(result.Steps, ActionStep{
			Name:    "Execute transaction",
			Status:  "skipped",
			Message: "dry-run mode (pass --execute to run)",
		})
		result.Executed = false
		result.Success = true
		return result, nil
	}

	// Step 4: Execute transaction
	result.Steps = append(result.Steps, ActionStep{Name: "Execute transaction", Status: "pending"})
	runner := oshelpers.NewRunner(false)

	// For multi-msg transactions, we need special handling
	if cmd.RequiresMultiMsg {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = "multi-message transaction requires manual composition"
		result.Warnings = append(result.Warnings,
			"Create-validator requires both MsgCreateValidator and MsgBurn in one tx.",
			"Run the commands below to generate unsigned JSON, then combine and sign:",
		)
		result.Error = fmt.Errorf("multi-message tx not yet automated; use generated commands manually")
		return result, result.Error
	}

	execResult := runner.RunTx(ctx, cmd.Binary, cmd.Args)
	result.Executed = true

	if !execResult.Success {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = execResult.Stderr
		result.Error = execResult.Error
		return result, result.Error
	}

	// Extract summary
	summary := oshelpers.ExtractTxSummary(execResult.Stdout)
	result.TxHash = summary.TxHash
	result.Height = summary.Height
	result.Success = summary.Success
	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("txhash: %s", summary.TxHash)

	return result, nil
}

// DelegateAction executes or previews a delegate transaction
func DelegateAction(ctx context.Context, opts ValidatorActionOptions, params DelegateParams) (*ValidatorActionResult, error) {
	result := &ValidatorActionResult{
		Action: TxActionDelegate,
		Steps:  make([]ActionStep, 0),
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Step 1: Validate network
	result.Steps = append(result.Steps, ActionStep{Name: "Validate network", Status: "pending"})
	network, err := GetNetwork(opts.Network)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Steps[len(result.Steps)-1].Status = "success"
	logger.Debug("network validated", "chain_id", network.ChainID)

	// Step 2: Build transaction command
	result.Steps = append(result.Steps, ActionStep{Name: "Build transaction", Status: "pending"})
	txOpts := opts.toTxBuilderOptions()
	cmd, err := BuildDelegateTx(txOpts, params)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Command = cmd
	result.Description = cmd.Description
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 3: Execute or skip
	return executeOrSkip(ctx, opts, result)
}

// UnbondAction executes or previews an unbond transaction
func UnbondAction(ctx context.Context, opts ValidatorActionOptions, params UnbondParams) (*ValidatorActionResult, error) {
	result := &ValidatorActionResult{
		Action: TxActionUnbond,
		Steps:  make([]ActionStep, 0),
	}

	// Step 1: Validate network
	result.Steps = append(result.Steps, ActionStep{Name: "Validate network", Status: "pending"})
	_, err := GetNetwork(opts.Network)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 2: Build transaction command
	result.Steps = append(result.Steps, ActionStep{Name: "Build transaction", Status: "pending"})
	txOpts := opts.toTxBuilderOptions()
	cmd, err := BuildUnbondTx(txOpts, params)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Command = cmd
	result.Description = cmd.Description
	result.Warnings = cmd.WarningMessages
	result.Steps[len(result.Steps)-1].Status = "success"

	return executeOrSkip(ctx, opts, result)
}

// RedelegateAction executes or previews a redelegate transaction
func RedelegateAction(ctx context.Context, opts ValidatorActionOptions, params RedelegateParams) (*ValidatorActionResult, error) {
	result := &ValidatorActionResult{
		Action: TxActionRedelegate,
		Steps:  make([]ActionStep, 0),
	}

	// Step 1: Validate network
	result.Steps = append(result.Steps, ActionStep{Name: "Validate network", Status: "pending"})
	_, err := GetNetwork(opts.Network)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 2: Build transaction command
	result.Steps = append(result.Steps, ActionStep{Name: "Build transaction", Status: "pending"})
	txOpts := opts.toTxBuilderOptions()
	cmd, err := BuildRedelegateTx(txOpts, params)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Command = cmd
	result.Description = cmd.Description
	result.Warnings = cmd.WarningMessages
	result.Steps[len(result.Steps)-1].Status = "success"

	return executeOrSkip(ctx, opts, result)
}

// WithdrawRewardsAction executes or previews a withdraw-rewards transaction
func WithdrawRewardsAction(ctx context.Context, opts ValidatorActionOptions, params WithdrawRewardsParams) (*ValidatorActionResult, error) {
	result := &ValidatorActionResult{
		Action: TxActionWithdrawRewards,
		Steps:  make([]ActionStep, 0),
	}

	// Step 1: Validate network
	result.Steps = append(result.Steps, ActionStep{Name: "Validate network", Status: "pending"})
	_, err := GetNetwork(opts.Network)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 2: Build transaction command
	result.Steps = append(result.Steps, ActionStep{Name: "Build transaction", Status: "pending"})
	txOpts := opts.toTxBuilderOptions()
	cmd, err := BuildWithdrawRewardsTx(txOpts, params)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Command = cmd
	result.Description = cmd.Description
	result.Steps[len(result.Steps)-1].Status = "success"

	return executeOrSkip(ctx, opts, result)
}

// VoteAction executes or previews a governance vote transaction
func VoteAction(ctx context.Context, opts ValidatorActionOptions, params VoteParams) (*ValidatorActionResult, error) {
	result := &ValidatorActionResult{
		Action: TxActionVote,
		Steps:  make([]ActionStep, 0),
	}

	// Step 1: Validate network
	result.Steps = append(result.Steps, ActionStep{Name: "Validate network", Status: "pending"})
	_, err := GetNetwork(opts.Network)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 2: Build transaction command
	result.Steps = append(result.Steps, ActionStep{Name: "Build transaction", Status: "pending"})
	txOpts := opts.toTxBuilderOptions()
	cmd, err := BuildVoteTx(txOpts, params)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Command = cmd
	result.Description = cmd.Description
	result.Steps[len(result.Steps)-1].Status = "success"

	return executeOrSkip(ctx, opts, result)
}

// executeOrSkip is a helper to execute tx or skip in dry-run mode
func executeOrSkip(ctx context.Context, opts ValidatorActionOptions, result *ValidatorActionResult) (*ValidatorActionResult, error) {
	if opts.DryRun || !opts.Execute {
		result.Steps = append(result.Steps, ActionStep{
			Name:    "Execute transaction",
			Status:  "skipped",
			Message: "dry-run mode (pass --execute to run)",
		})
		result.Executed = false
		result.Success = true
		return result, nil
	}

	// Execute transaction
	result.Steps = append(result.Steps, ActionStep{Name: "Execute transaction", Status: "pending"})
	runner := oshelpers.NewRunner(false)

	execResult := runner.RunTx(ctx, result.Command.Binary, result.Command.Args)
	result.Executed = true

	if !execResult.Success {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = execResult.Stderr
		result.Error = execResult.Error
		return result, result.Error
	}

	// Extract summary
	summary := oshelpers.ExtractTxSummary(execResult.Stdout)
	result.TxHash = summary.TxHash
	result.Height = summary.Height
	result.Success = summary.Success
	result.Steps[len(result.Steps)-1].Status = "success"
	if summary.TxHash != "" {
		result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("txhash: %s", summary.TxHash)
	}

	return result, nil
}

// BankSendAction executes or previews a bank send transaction
func BankSendAction(ctx context.Context, opts ValidatorActionOptions, params BankSendParams) (*ValidatorActionResult, error) {
	result := &ValidatorActionResult{
		Action: TxActionSend,
		Steps:  make([]ActionStep, 0),
	}

	// Step 1: Validate network
	result.Steps = append(result.Steps, ActionStep{Name: "Validate network", Status: "pending"})
	_, err := GetNetwork(opts.Network)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 2: Build transaction command
	result.Steps = append(result.Steps, ActionStep{Name: "Build transaction", Status: "pending"})
	txOpts := opts.toTxBuilderOptions()
	cmd, err := BuildBankSendTx(txOpts, params)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result, err
	}
	result.Command = cmd
	result.Description = cmd.Description
	result.Steps[len(result.Steps)-1].Status = "success"

	return executeOrSkip(ctx, opts, result)
}

// CheckRPCBeforeAction validates RPC connectivity before an action
func CheckRPCBeforeAction(opts ValidatorActionOptions) (*RPCCheckResult, error) {
	network, err := GetNetwork(opts.Network)
	if err != nil {
		return nil, err
	}

	// Determine endpoint
	nodeURL := opts.Node
	if nodeURL == "" {
		nodeURL = "http://localhost:26657"
	}

	endpoints := Endpoints{
		CometRPC: nodeURL,
	}

	// Quick RPC check
	results := CheckRPC(opts.Network, endpoints)
	if len(results.Results) > 0 {
		cometResult := results.Results[0]
		if cometResult.Status == "FAIL" {
			return &cometResult, fmt.Errorf("RPC check failed: %s", cometResult.Message)
		}

		// Chain ID is verified by CheckRPC internally and reported in Details
		// If the check passed, the chain-id matched
		_ = network // Use network to avoid unused variable warning

		return &cometResult, nil
	}

	return nil, fmt.Errorf("no RPC check results")
}

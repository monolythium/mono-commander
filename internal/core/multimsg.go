// Package core provides the core logic for mono-commander.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	oshelpers "github.com/monolythium/mono-commander/internal/os"
)

// TxJSON represents a Cosmos SDK transaction JSON structure
type TxJSON struct {
	Body      TxBody   `json:"body"`
	AuthInfo  AuthInfo `json:"auth_info"`
	Signatures []string `json:"signatures"`
}

// TxBody represents the body of a transaction
type TxBody struct {
	Messages                    []json.RawMessage `json:"messages"`
	Memo                        string            `json:"memo"`
	TimeoutHeight               string            `json:"timeout_height"`
	ExtensionOptions            []json.RawMessage `json:"extension_options"`
	NonCriticalExtensionOptions []json.RawMessage `json:"non_critical_extension_options"`
}

// AuthInfo represents the auth info of a transaction
type AuthInfo struct {
	SignerInfos []json.RawMessage `json:"signer_infos"`
	Fee         TxFee             `json:"fee"`
	Tip         json.RawMessage   `json:"tip,omitempty"`
}

// TxFee represents the fee structure
type TxFee struct {
	Amount   []Coin `json:"amount"`
	GasLimit string `json:"gas_limit"`
	Payer    string `json:"payer"`
	Granter  string `json:"granter"`
}

// Coin represents a coin amount
type Coin struct {
	Denom  string `json:"denom"`
	Amount string `json:"amount"`
}

// MultiMsgExecutor handles multi-message transaction composition and execution
type MultiMsgExecutor struct {
	Binary         string // monod binary path
	Home           string // home directory
	ChainID        string // chain ID
	From           string // from key/address
	KeyringBackend string // keyring backend
	Node           string // RPC node URL
	GasAdjustment  float64 // gas adjustment multiplier (default 1.5)
}

// NewMultiMsgExecutor creates a new multi-message executor
func NewMultiMsgExecutor(opts TxBuilderOptions, network Network) *MultiMsgExecutor {
	chainID := opts.ChainID
	if chainID == "" {
		chainID = network.ChainID
	}

	return &MultiMsgExecutor{
		Binary:         "monod",
		Home:           opts.Home,
		ChainID:        chainID,
		From:           opts.From,
		KeyringBackend: opts.KeyringBackend,
		Node:           opts.Node,
		GasAdjustment:  1.5,
	}
}

// GenerateUnsignedTx generates an unsigned transaction JSON from a command
func (e *MultiMsgExecutor) GenerateUnsignedTx(ctx context.Context, args []string) ([]byte, error) {
	runner := oshelpers.NewRunner(false)
	runner.Timeout = 30 * 1000000000 // 30 seconds

	// Ensure --generate-only is in args
	hasGenerateOnly := false
	for _, arg := range args {
		if arg == "--generate-only" {
			hasGenerateOnly = true
			break
		}
	}
	if !hasGenerateOnly {
		args = append(args, "--generate-only")
	}

	result := runner.Run(ctx, e.Binary, args)
	if !result.Success {
		return nil, fmt.Errorf("failed to generate unsigned tx: %s", result.Stderr)
	}

	// The stdout should contain the unsigned tx JSON
	return []byte(result.Stdout), nil
}

// CombineMessages combines multiple unsigned transaction JSONs into one
func (e *MultiMsgExecutor) CombineMessages(txJSONs ...[]byte) ([]byte, error) {
	if len(txJSONs) == 0 {
		return nil, fmt.Errorf("no transactions to combine")
	}
	if len(txJSONs) == 1 {
		return txJSONs[0], nil
	}

	// Parse first transaction as base
	var combinedTx TxJSON
	if err := json.Unmarshal(txJSONs[0], &combinedTx); err != nil {
		return nil, fmt.Errorf("failed to parse base transaction: %w", err)
	}

	// Track total gas
	totalGas := int64(0)
	if combinedTx.AuthInfo.Fee.GasLimit != "" {
		gas, _ := strconv.ParseInt(combinedTx.AuthInfo.Fee.GasLimit, 10, 64)
		totalGas += gas
	}

	// Combine messages from all transactions
	for i := 1; i < len(txJSONs); i++ {
		var tx TxJSON
		if err := json.Unmarshal(txJSONs[i], &tx); err != nil {
			return nil, fmt.Errorf("failed to parse transaction %d: %w", i, err)
		}

		// Append messages
		combinedTx.Body.Messages = append(combinedTx.Body.Messages, tx.Body.Messages...)

		// Accumulate gas
		if tx.AuthInfo.Fee.GasLimit != "" {
			gas, _ := strconv.ParseInt(tx.AuthInfo.Fee.GasLimit, 10, 64)
			totalGas += gas
		}
	}

	// Apply gas adjustment
	adjustedGas := int64(float64(totalGas) * e.GasAdjustment)
	combinedTx.AuthInfo.Fee.GasLimit = strconv.FormatInt(adjustedGas, 10)

	return json.MarshalIndent(combinedTx, "", "  ")
}

// SignTx signs an unsigned transaction
func (e *MultiMsgExecutor) SignTx(ctx context.Context, unsignedTxJSON []byte) ([]byte, error) {
	// Write unsigned tx to temp file
	tmpDir := os.TempDir()
	unsignedPath := filepath.Join(tmpDir, "unsigned_tx.json")
	signedPath := filepath.Join(tmpDir, "signed_tx.json")

	if err := os.WriteFile(unsignedPath, unsignedTxJSON, 0600); err != nil {
		return nil, fmt.Errorf("failed to write unsigned tx: %w", err)
	}
	defer os.Remove(unsignedPath)
	defer os.Remove(signedPath)

	// Build sign command
	args := []string{"tx", "sign", unsignedPath,
		"--from", e.From,
		"--chain-id", e.ChainID,
		"--output-document", signedPath,
	}

	if e.Home != "" {
		args = append(args, "--home", e.Home)
	}
	if e.KeyringBackend != "" {
		args = append(args, "--keyring-backend", e.KeyringBackend)
	}
	if e.Node != "" {
		args = append(args, "--node", e.Node)
	}

	// Don't skip signature verification by default
	args = append(args, "--offline=false")

	runner := oshelpers.NewRunner(false)
	runner.Timeout = 60 * 1000000000 // 60 seconds for signing

	result := runner.Run(ctx, e.Binary, args)
	if !result.Success {
		return nil, fmt.Errorf("failed to sign tx: %s", result.Stderr)
	}

	// Read signed transaction
	signedTx, err := os.ReadFile(signedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read signed tx: %w", err)
	}

	return signedTx, nil
}

// BroadcastTx broadcasts a signed transaction
func (e *MultiMsgExecutor) BroadcastTx(ctx context.Context, signedTxJSON []byte) (*oshelpers.TxSummary, error) {
	// Write signed tx to temp file
	tmpDir := os.TempDir()
	signedPath := filepath.Join(tmpDir, "broadcast_tx.json")

	if err := os.WriteFile(signedPath, signedTxJSON, 0600); err != nil {
		return nil, fmt.Errorf("failed to write signed tx: %w", err)
	}
	defer os.Remove(signedPath)

	// Build broadcast command
	args := []string{"tx", "broadcast", signedPath, "--broadcast-mode", "sync"}

	if e.Node != "" {
		args = append(args, "--node", e.Node)
	}

	runner := oshelpers.NewRunner(false)
	runner.Timeout = 60 * 1000000000 // 60 seconds for broadcast

	result := runner.Run(ctx, e.Binary, args)
	if !result.Success {
		return nil, fmt.Errorf("failed to broadcast tx: %s", result.Stderr)
	}

	// Extract transaction summary
	summary := oshelpers.ExtractTxSummary(result.Stdout)

	// Check if broadcast was successful
	if !summary.Success && summary.Code != 0 {
		return summary, fmt.Errorf("tx broadcast failed with code %d: %s", summary.Code, summary.RawLog)
	}

	return summary, nil
}

// ExecuteMultiMsg executes a multi-message transaction
func (e *MultiMsgExecutor) ExecuteMultiMsg(ctx context.Context, commands []*TxCommand) (*oshelpers.TxSummary, error) {
	if len(commands) == 0 {
		return nil, fmt.Errorf("no commands to execute")
	}

	// Generate unsigned transactions for each command
	unsignedTxs := make([][]byte, 0, len(commands))
	for i, cmd := range commands {
		unsignedTx, err := e.GenerateUnsignedTx(ctx, cmd.Args)
		if err != nil {
			return nil, fmt.Errorf("failed to generate unsigned tx for command %d (%s): %w", i, cmd.Action, err)
		}
		unsignedTxs = append(unsignedTxs, unsignedTx)
	}

	// Combine all messages into single transaction
	combinedTx, err := e.CombineMessages(unsignedTxs...)
	if err != nil {
		return nil, fmt.Errorf("failed to combine transactions: %w", err)
	}

	// Sign the combined transaction
	signedTx, err := e.SignTx(ctx, combinedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign combined tx: %w", err)
	}

	// Broadcast the signed transaction
	summary, err := e.BroadcastTx(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return summary, nil
}

// GetMultiMsgPreviewCommands returns the commands that would be used for a multi-msg tx
func GetMultiMsgPreviewCommands(cmd *TxCommand) []string {
	if !cmd.RequiresMultiMsg || len(cmd.MultiMsgCommands) == 0 {
		return []string{cmd.String()}
	}

	preview := []string{
		"# Multi-message transaction preview:",
		"# Step 1: Generate unsigned transactions for each message",
	}

	for i, subCmd := range cmd.MultiMsgCommands {
		preview = append(preview, fmt.Sprintf("# Message %d: %s", i+1, subCmd.Description))
		preview = append(preview, subCmd.String())
		preview = append(preview, "")
	}

	preview = append(preview, "# Step 2: Combine messages, sign, and broadcast")
	preview = append(preview, "# (monoctl handles this automatically with --execute)")

	return preview
}

// FormatMultiMsgDryRun formats a dry-run output for multi-message transactions
func FormatMultiMsgDryRun(cmd *TxCommand) string {
	var sb strings.Builder

	sb.WriteString("=== Multi-Message Transaction ===\n\n")
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", cmd.Description))

	if len(cmd.WarningMessages) > 0 {
		sb.WriteString("⚠️  Warnings:\n")
		for _, w := range cmd.WarningMessages {
			sb.WriteString(fmt.Sprintf("   • %s\n", w))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Messages to be combined:\n\n")

	for i, subCmd := range cmd.MultiMsgCommands {
		sb.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, subCmd.Description))
		sb.WriteString(fmt.Sprintf("      %s\n\n", subCmd.String()))
	}

	sb.WriteString("To execute this transaction, run with --execute flag.\n")

	return sb.String()
}

// Package core provides the core logic for mono-commander.
package core

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// Denom constants
const (
	// BaseDenom is the smallest unit denomination (18 decimals)
	BaseDenom = "alyth"
	// DisplayDenom is the user-facing denomination
	DisplayDenom = "LYTH"
	// Decimals is the number of decimal places
	Decimals = 18
)

// Validator constants (from blueprint)
const (
	// MinSelfDelegationLYTH is the minimum self-delegation in LYTH
	MinSelfDelegationLYTH = 100000
	// ValidatorBurnLYTH is the required burn amount for validator creation in LYTH
	ValidatorBurnLYTH = 100000
)

// MinSelfDelegationAlyth is the minimum self-delegation in alyth (100,000 LYTH * 10^18)
var MinSelfDelegationAlyth = new(big.Int).Mul(
	big.NewInt(MinSelfDelegationLYTH),
	new(big.Int).Exp(big.NewInt(10), big.NewInt(Decimals), nil),
)

// ValidatorBurnAlyth is the required burn amount in alyth (100,000 LYTH * 10^18)
var ValidatorBurnAlyth = new(big.Int).Mul(
	big.NewInt(ValidatorBurnLYTH),
	new(big.Int).Exp(big.NewInt(10), big.NewInt(Decimals), nil),
)

// Bech32 prefixes
const (
	Bech32PrefixAccAddr  = "mono"
	Bech32PrefixValAddr  = "monovaloper"
	Bech32PrefixConsAddr = "monovalcons"
)

// Address validation patterns
var (
	// AccountAddressPattern matches mono1... addresses (mono + 39 chars)
	AccountAddressPattern = regexp.MustCompile(`^mono1[a-z0-9]{38}$`)
	// ValoperAddressPattern matches monovaloper1... addresses
	ValoperAddressPattern = regexp.MustCompile(`^monovaloper1[a-z0-9]{38}$`)
)

// TxAction represents a transaction action type
type TxAction string

const (
	TxActionCreateValidator TxAction = "create-validator"
	TxActionDelegate        TxAction = "delegate"
	TxActionUnbond          TxAction = "unbond"
	TxActionRedelegate      TxAction = "redelegate"
	TxActionWithdrawRewards TxAction = "withdraw-rewards"
	TxActionWithdrawComm    TxAction = "withdraw-commission"
	TxActionVote            TxAction = "vote"
)

// VoteOption represents a governance vote option
type VoteOption string

const (
	VoteYes        VoteOption = "yes"
	VoteNo         VoteOption = "no"
	VoteAbstain    VoteOption = "abstain"
	VoteNoWithVeto VoteOption = "no_with_veto"
)

// TxCommand represents a generated monod transaction command
type TxCommand struct {
	// Action is the type of transaction
	Action TxAction
	// Binary is the monod binary path (default: "monod")
	Binary string
	// Args is the list of command arguments
	Args []string
	// Description is a human-readable description
	Description string
	// WarningMessages contains any warnings about the command
	WarningMessages []string
	// RequiresMultiMsg indicates if this requires multi-message tx
	RequiresMultiMsg bool
	// MultiMsgCommands contains individual commands for multi-msg tx
	// When RequiresMultiMsg is true, these are the separate messages
	MultiMsgCommands []*TxCommand
}

// String returns the full command as a string
func (c *TxCommand) String() string {
	if c.Binary == "" {
		c.Binary = "monod"
	}
	return c.Binary + " " + strings.Join(c.Args, " ")
}

// TxBuilderOptions contains common options for all tx builders
type TxBuilderOptions struct {
	Network   NetworkName
	Home      string
	From      string // key name or address
	Fees      string // amount in alyth (e.g., "10000alyth")
	GasPrices string // price in alyth (e.g., "0.025alyth")
	Gas       string // gas limit or "auto"
	Node      string // RPC node URL
	ChainID   string // chain-id override (uses network default if empty)
	Broadcast bool   // whether to broadcast (default: false for dry-run)
	DryRun    bool   // dry-run mode (default: true)
}

// ValidateAddress validates a Monolythium account address (mono1...)
func ValidateAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("address is required")
	}
	if !AccountAddressPattern.MatchString(addr) {
		return fmt.Errorf("invalid address format: must be mono1... (got %s)", addr)
	}
	return nil
}

// ValidateValoperAddress validates a Monolythium validator operator address
func ValidateValoperAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("validator operator address is required")
	}
	if !ValoperAddressPattern.MatchString(addr) {
		return fmt.Errorf("invalid validator operator address format: must be monovaloper1... (got %s)", addr)
	}
	return nil
}

// ValidateAmount validates an amount string in alyth format
func ValidateAmount(amount string) error {
	if amount == "" {
		return fmt.Errorf("amount is required")
	}

	// Must end with alyth
	if !strings.HasSuffix(amount, BaseDenom) {
		return fmt.Errorf("amount must be in %s (got %s)", BaseDenom, amount)
	}

	// Extract numeric part
	numStr := strings.TrimSuffix(amount, BaseDenom)
	if numStr == "" {
		return fmt.Errorf("amount value is required")
	}

	// Validate it's a valid integer
	value := new(big.Int)
	_, ok := value.SetString(numStr, 10)
	if !ok {
		return fmt.Errorf("invalid amount value: %s", numStr)
	}

	// Must be positive
	if value.Sign() <= 0 {
		return fmt.Errorf("amount must be positive (got %s)", numStr)
	}

	return nil
}

// ValidateMinSelfDelegation validates the minimum self-delegation amount
func ValidateMinSelfDelegation(amount string) error {
	if err := ValidateAmount(amount); err != nil {
		return err
	}

	// Extract numeric part
	numStr := strings.TrimSuffix(amount, BaseDenom)
	value := new(big.Int)
	value.SetString(numStr, 10)

	// Must be >= MinSelfDelegationAlyth
	if value.Cmp(MinSelfDelegationAlyth) < 0 {
		return fmt.Errorf("minimum self-delegation must be at least %d LYTH (%s alyth), got %s",
			MinSelfDelegationLYTH, MinSelfDelegationAlyth.String(), numStr)
	}

	return nil
}

// ParseAmount parses an amount string and returns the big.Int value
func ParseAmount(amount string) (*big.Int, error) {
	if err := ValidateAmount(amount); err != nil {
		return nil, err
	}
	numStr := strings.TrimSuffix(amount, BaseDenom)
	value := new(big.Int)
	value.SetString(numStr, 10)
	return value, nil
}

// FormatLYTH formats an alyth amount as LYTH for display
func FormatLYTH(alyth *big.Int) string {
	if alyth == nil {
		return "0 LYTH"
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(Decimals), nil)
	lyth := new(big.Int).Div(alyth, divisor)
	return fmt.Sprintf("%s LYTH", lyth.String())
}

// LYTHToAlyth converts LYTH to alyth
func LYTHToAlyth(lyth int64) string {
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(Decimals), nil)
	result := new(big.Int).Mul(big.NewInt(lyth), multiplier)
	return result.String() + BaseDenom
}

// ValidateVoteOption validates a governance vote option
func ValidateVoteOption(option string) (VoteOption, error) {
	switch strings.ToLower(option) {
	case "yes", "1":
		return VoteYes, nil
	case "no", "2":
		return VoteNo, nil
	case "abstain", "3":
		return VoteAbstain, nil
	case "no_with_veto", "nowithveto", "4":
		return VoteNoWithVeto, nil
	default:
		return "", fmt.Errorf("invalid vote option: %s (valid: yes, no, abstain, no_with_veto)", option)
	}
}

// ValidateCommissionRate validates a commission rate (0.0 to 1.0)
func ValidateCommissionRate(rate string) error {
	if rate == "" {
		return fmt.Errorf("commission rate is required")
	}
	// Parse as float to validate format
	var f float64
	_, err := fmt.Sscanf(rate, "%f", &f)
	if err != nil {
		return fmt.Errorf("invalid commission rate format: %s", rate)
	}
	if f < 0 || f > 1 {
		return fmt.Errorf("commission rate must be between 0.0 and 1.0 (got %s)", rate)
	}
	return nil
}

// buildCommonArgs builds common transaction arguments
func buildCommonArgs(opts TxBuilderOptions, network Network) []string {
	args := []string{}

	// Chain ID
	chainID := opts.ChainID
	if chainID == "" {
		chainID = network.ChainID
	}
	args = append(args, "--chain-id", chainID)

	// Home directory
	if opts.Home != "" {
		args = append(args, "--home", opts.Home)
	}

	// From key/address
	if opts.From != "" {
		args = append(args, "--from", opts.From)
	}

	// Fees or gas prices
	if opts.Fees != "" {
		args = append(args, "--fees", opts.Fees)
	} else if opts.GasPrices != "" {
		args = append(args, "--gas-prices", opts.GasPrices)
		if opts.Gas == "" {
			opts.Gas = "auto"
		}
	}

	// Gas
	if opts.Gas != "" {
		args = append(args, "--gas", opts.Gas)
	}

	// Node RPC
	if opts.Node != "" {
		args = append(args, "--node", opts.Node)
	}

	// Broadcast mode
	if opts.Broadcast {
		args = append(args, "--broadcast-mode", "sync")
		args = append(args, "-y") // skip confirmation
	} else {
		// Generate only mode (for dry-run display)
		args = append(args, "--generate-only")
	}

	return args
}

// CreateValidatorParams contains parameters for create-validator
type CreateValidatorParams struct {
	Moniker               string
	Identity              string // optional keybase identity
	Website               string // optional website
	SecurityContact       string // optional security contact
	Details               string // optional details
	CommissionRate        string // e.g., "0.10" for 10%
	CommissionMaxRate     string // e.g., "0.20" for 20%
	CommissionMaxChange   string // e.g., "0.01" for 1%
	MinSelfDelegation     string // in alyth, must be >= 100000 LYTH
	Amount                string // self-bond amount in alyth, must be >= 100000 LYTH
	PubKeyPath            string // optional: path to consensus pubkey file
	PubKeyAuto            bool   // if true, derive from keyring
}

// BuildCreateValidatorTx builds a create-validator transaction command.
// Note: Per blueprint, this requires BOTH MsgCreateValidator AND MsgBurn(100k LYTH)
// in the same transaction. This function generates commands that can be composed.
func BuildCreateValidatorTx(opts TxBuilderOptions, params CreateValidatorParams) (*TxCommand, error) {
	// Validate required fields
	if params.Moniker == "" {
		return nil, fmt.Errorf("moniker is required")
	}
	if err := ValidateMinSelfDelegation(params.Amount); err != nil {
		return nil, fmt.Errorf("self-bond amount: %w", err)
	}
	if err := ValidateMinSelfDelegation(params.MinSelfDelegation); err != nil {
		return nil, fmt.Errorf("min-self-delegation: %w", err)
	}
	if err := ValidateCommissionRate(params.CommissionRate); err != nil {
		return nil, err
	}
	if err := ValidateCommissionRate(params.CommissionMaxRate); err != nil {
		return nil, fmt.Errorf("commission-max-rate: %w", err)
	}
	if err := ValidateCommissionRate(params.CommissionMaxChange); err != nil {
		return nil, fmt.Errorf("commission-max-change-rate: %w", err)
	}

	// Get network config
	network, err := GetNetwork(opts.Network)
	if err != nil {
		return nil, err
	}

	// Build the create-validator message args
	createValArgs := []string{"tx", "staking", "create-validator"}
	createValArgs = append(createValArgs, "--moniker", params.Moniker)
	createValArgs = append(createValArgs, "--amount", params.Amount)
	createValArgs = append(createValArgs, "--min-self-delegation", strings.TrimSuffix(params.MinSelfDelegation, BaseDenom))
	createValArgs = append(createValArgs, "--commission-rate", params.CommissionRate)
	createValArgs = append(createValArgs, "--commission-max-rate", params.CommissionMaxRate)
	createValArgs = append(createValArgs, "--commission-max-change-rate", params.CommissionMaxChange)

	// Optional fields
	if params.Identity != "" {
		createValArgs = append(createValArgs, "--identity", params.Identity)
	}
	if params.Website != "" {
		createValArgs = append(createValArgs, "--website", params.Website)
	}
	if params.SecurityContact != "" {
		createValArgs = append(createValArgs, "--security-contact", params.SecurityContact)
	}
	if params.Details != "" {
		createValArgs = append(createValArgs, "--details", params.Details)
	}

	// Pubkey handling
	if params.PubKeyPath != "" {
		createValArgs = append(createValArgs, "--pubkey", fmt.Sprintf("$(cat %s)", params.PubKeyPath))
	}
	// If PubKeyAuto is true, monod will derive from keyring (default behavior)

	// Add common args
	createValArgs = append(createValArgs, buildCommonArgs(opts, network)...)

	// Build the burn message for 100k LYTH
	burnArgs := []string{"tx", "bank", "burn", ValidatorBurnAlyth.String() + BaseDenom}
	burnArgs = append(burnArgs, buildCommonArgs(opts, network)...)

	// Create the compound command
	cmd := &TxCommand{
		Action:           TxActionCreateValidator,
		Binary:           "monod",
		RequiresMultiMsg: true,
		Description:      fmt.Sprintf("Create validator %s with %s self-bond (includes %d LYTH burn)", params.Moniker, FormatLYTH(mustParseAmount(params.Amount)), ValidatorBurnLYTH),
		WarningMessages: []string{
			fmt.Sprintf("IMPORTANT: Validator creation requires burning %d LYTH in the same transaction.", ValidatorBurnLYTH),
			"This is a one-time, non-refundable burn as per Monolythium blueprint.",
		},
		MultiMsgCommands: []*TxCommand{
			{
				Action:      TxActionCreateValidator,
				Binary:      "monod",
				Args:        createValArgs,
				Description: "Create validator message",
			},
			{
				Action:      TxAction("burn"),
				Binary:      "monod",
				Args:        burnArgs,
				Description: fmt.Sprintf("Burn %d LYTH (required for validator creation)", ValidatorBurnLYTH),
			},
		},
	}

	// For display purposes, show combined command approach
	// Note: monod may or may not support --generate-only with multi-msg
	// We provide individual commands that can be combined via unsigned tx JSON
	cmd.Args = createValArgs // Primary command for display

	return cmd, nil
}

// mustParseAmount is a helper that panics on invalid amount (for known-good values)
func mustParseAmount(amount string) *big.Int {
	v, err := ParseAmount(amount)
	if err != nil {
		return big.NewInt(0)
	}
	return v
}

// DelegateParams contains parameters for delegation
type DelegateParams struct {
	ValidatorAddr string // monovaloper1... address
	Amount        string // in alyth
}

// BuildDelegateTx builds a delegate transaction command
func BuildDelegateTx(opts TxBuilderOptions, params DelegateParams) (*TxCommand, error) {
	if err := ValidateValoperAddress(params.ValidatorAddr); err != nil {
		return nil, err
	}
	if err := ValidateAmount(params.Amount); err != nil {
		return nil, err
	}

	network, err := GetNetwork(opts.Network)
	if err != nil {
		return nil, err
	}

	args := []string{"tx", "staking", "delegate", params.ValidatorAddr, params.Amount}
	args = append(args, buildCommonArgs(opts, network)...)

	return &TxCommand{
		Action:      TxActionDelegate,
		Binary:      "monod",
		Args:        args,
		Description: fmt.Sprintf("Delegate %s to %s", FormatLYTH(mustParseAmount(params.Amount)), params.ValidatorAddr),
	}, nil
}

// UnbondParams contains parameters for unbonding
type UnbondParams struct {
	ValidatorAddr string // monovaloper1... address
	Amount        string // in alyth
}

// BuildUnbondTx builds an unbond transaction command
func BuildUnbondTx(opts TxBuilderOptions, params UnbondParams) (*TxCommand, error) {
	if err := ValidateValoperAddress(params.ValidatorAddr); err != nil {
		return nil, err
	}
	if err := ValidateAmount(params.Amount); err != nil {
		return nil, err
	}

	network, err := GetNetwork(opts.Network)
	if err != nil {
		return nil, err
	}

	args := []string{"tx", "staking", "unbond", params.ValidatorAddr, params.Amount}
	args = append(args, buildCommonArgs(opts, network)...)

	return &TxCommand{
		Action:      TxActionUnbond,
		Binary:      "monod",
		Args:        args,
		Description: fmt.Sprintf("Unbond %s from %s (3-day unbonding period)", FormatLYTH(mustParseAmount(params.Amount)), params.ValidatorAddr),
		WarningMessages: []string{
			"Unbonded tokens will be available after the 3-day unbonding period.",
		},
	}, nil
}

// RedelegateParams contains parameters for redelegation
type RedelegateParams struct {
	SrcValidatorAddr string // source monovaloper1... address
	DstValidatorAddr string // destination monovaloper1... address
	Amount           string // in alyth
}

// BuildRedelegateTx builds a redelegate transaction command
func BuildRedelegateTx(opts TxBuilderOptions, params RedelegateParams) (*TxCommand, error) {
	if err := ValidateValoperAddress(params.SrcValidatorAddr); err != nil {
		return nil, fmt.Errorf("source validator: %w", err)
	}
	if err := ValidateValoperAddress(params.DstValidatorAddr); err != nil {
		return nil, fmt.Errorf("destination validator: %w", err)
	}
	if err := ValidateAmount(params.Amount); err != nil {
		return nil, err
	}

	network, err := GetNetwork(opts.Network)
	if err != nil {
		return nil, err
	}

	args := []string{"tx", "staking", "redelegate", params.SrcValidatorAddr, params.DstValidatorAddr, params.Amount}
	args = append(args, buildCommonArgs(opts, network)...)

	return &TxCommand{
		Action:      TxActionRedelegate,
		Binary:      "monod",
		Args:        args,
		Description: fmt.Sprintf("Redelegate %s from %s to %s", FormatLYTH(mustParseAmount(params.Amount)), params.SrcValidatorAddr, params.DstValidatorAddr),
		WarningMessages: []string{
			"Redelegation is instant but you cannot redelegate the same tokens again for 3 days.",
		},
	}, nil
}

// WithdrawRewardsParams contains parameters for withdrawing rewards
type WithdrawRewardsParams struct {
	ValidatorAddr string // optional: specific validator, empty = all
	Commission    bool   // if true, also withdraw commission (for validator operators)
}

// BuildWithdrawRewardsTx builds a withdraw-rewards transaction command
func BuildWithdrawRewardsTx(opts TxBuilderOptions, params WithdrawRewardsParams) (*TxCommand, error) {
	network, err := GetNetwork(opts.Network)
	if err != nil {
		return nil, err
	}

	var args []string
	var description string

	if params.Commission {
		// Withdraw commission (validator operator only)
		if params.ValidatorAddr != "" {
			if err := ValidateValoperAddress(params.ValidatorAddr); err != nil {
				return nil, err
			}
		}
		args = []string{"tx", "distribution", "withdraw-rewards"}
		if params.ValidatorAddr != "" {
			args = append(args, params.ValidatorAddr)
		}
		args = append(args, "--commission")
		description = "Withdraw validator commission"
		if params.ValidatorAddr != "" {
			description += fmt.Sprintf(" from %s", params.ValidatorAddr)
		}
	} else if params.ValidatorAddr != "" {
		// Withdraw from specific validator
		if err := ValidateValoperAddress(params.ValidatorAddr); err != nil {
			return nil, err
		}
		args = []string{"tx", "distribution", "withdraw-rewards", params.ValidatorAddr}
		description = fmt.Sprintf("Withdraw staking rewards from %s", params.ValidatorAddr)
	} else {
		// Withdraw all rewards
		args = []string{"tx", "distribution", "withdraw-all-rewards"}
		description = "Withdraw all staking rewards"
	}

	args = append(args, buildCommonArgs(opts, network)...)

	return &TxCommand{
		Action:      TxActionWithdrawRewards,
		Binary:      "monod",
		Args:        args,
		Description: description,
	}, nil
}

// VoteParams contains parameters for governance voting
type VoteParams struct {
	ProposalID string     // proposal ID (numeric)
	Option     VoteOption // vote option
}

// BuildVoteTx builds a governance vote transaction command
func BuildVoteTx(opts TxBuilderOptions, params VoteParams) (*TxCommand, error) {
	if params.ProposalID == "" {
		return nil, fmt.Errorf("proposal ID is required")
	}

	// Validate proposal ID is numeric
	var propID int
	if _, err := fmt.Sscanf(params.ProposalID, "%d", &propID); err != nil {
		return nil, fmt.Errorf("invalid proposal ID: %s (must be numeric)", params.ProposalID)
	}
	if propID <= 0 {
		return nil, fmt.Errorf("proposal ID must be positive")
	}

	network, err := GetNetwork(opts.Network)
	if err != nil {
		return nil, err
	}

	args := []string{"tx", "gov", "vote", params.ProposalID, string(params.Option)}
	args = append(args, buildCommonArgs(opts, network)...)

	return &TxCommand{
		Action:      TxActionVote,
		Binary:      "monod",
		Args:        args,
		Description: fmt.Sprintf("Vote %s on proposal #%s", params.Option, params.ProposalID),
	}, nil
}

// Package main provides the CLI entry point for mono-commander.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/monolythium/mono-commander/internal/core"
	"github.com/monolythium/mono-commander/internal/logs"
	"github.com/monolythium/mono-commander/internal/mesh"
	"github.com/monolythium/mono-commander/internal/net"
	oshelpers "github.com/monolythium/mono-commander/internal/os"
	"github.com/monolythium/mono-commander/internal/tui"
	"github.com/monolythium/mono-commander/internal/update"
	"github.com/monolythium/mono-commander/internal/walletgen"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Common flag variables for M4 commands
var (
	// Shared tx flags
	txNetwork   string
	txHome      string
	txFrom      string
	txFees      string
	txGasPrices string
	txGas       string
	txNode      string
	txChainID   string
	txDryRun    bool
	txExecute   bool
)

var (
	// Global flags
	jsonOutput bool
	verbose    bool

	// Root command
	rootCmd = &cobra.Command{
		Use:   "monoctl",
		Short: "Mono Commander - Monolythium node management tool",
		Long: `Mono Commander is a TUI-first tool for installing and operating
Monolythium nodes across Localnet, Sprintnet, Testnet, and Mainnet.

Start the interactive TUI:
  monoctl

Or use CLI commands:
  monoctl networks list
  monoctl join --network Sprintnet --home ~/.monod
  monoctl systemd install --network Sprintnet --home ~/.monod --user monod`,
		Run: func(cmd *cobra.Command, args []string) {
			// Default: launch TUI
			if err := tui.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Networks command
	networksCmd = &cobra.Command{
		Use:   "networks",
		Short: "Network management commands",
	}

	networksListCmd = &cobra.Command{
		Use:   "list",
		Short: "List supported networks",
		Run:   runNetworksList,
	}

	// Join command
	joinCmd = &cobra.Command{
		Use:   "join",
		Short: "Join a network (download genesis, configure peers)",
		Run:   runJoin,
	}

	// Peers command
	peersCmd = &cobra.Command{
		Use:   "peers",
		Short: "Peer management commands",
	}

	peersUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update peers from registry",
		Run:   runPeersUpdate,
	}

	// Systemd command
	systemdCmd = &cobra.Command{
		Use:   "systemd",
		Short: "Systemd unit management",
	}

	systemdInstallCmd = &cobra.Command{
		Use:   "install",
		Short: "Generate systemd unit file",
		Run:   runSystemdInstall,
	}

	// Status command
	statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show node status",
		Run:   runStatus,
	}

	// RPC command
	rpcCmd = &cobra.Command{
		Use:   "rpc",
		Short: "RPC health checks",
	}

	rpcCheckCmd = &cobra.Command{
		Use:   "check",
		Short: "Check RPC endpoint health",
		Run:   runRPCCheck,
	}

	// Logs command
	logsCmd = &cobra.Command{
		Use:   "logs",
		Short: "Tail node logs",
		Run:   runLogs,
	}

	// M4: Validator command group
	validatorCmd = &cobra.Command{
		Use:   "validator",
		Short: "Validator operations",
	}

	validatorCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new validator (includes required 100k LYTH burn)",
		Run:   runValidatorCreate,
	}

	// M4: Stake command group
	stakeCmd = &cobra.Command{
		Use:   "stake",
		Short: "Staking operations (delegate, unbond, redelegate)",
	}

	stakeDelegateCmd = &cobra.Command{
		Use:   "delegate",
		Short: "Delegate tokens to a validator",
		Run:   runStakeDelegate,
	}

	stakeUnbondCmd = &cobra.Command{
		Use:   "unbond",
		Short: "Unbond tokens from a validator",
		Run:   runStakeUnbond,
	}

	stakeRedelegateCmd = &cobra.Command{
		Use:   "redelegate",
		Short: "Redelegate tokens between validators",
		Run:   runStakeRedelegate,
	}

	// M4: Rewards command group
	rewardsCmd = &cobra.Command{
		Use:   "rewards",
		Short: "Rewards operations",
	}

	rewardsWithdrawCmd = &cobra.Command{
		Use:   "withdraw",
		Short: "Withdraw staking rewards or validator commission",
		Run:   runRewardsWithdraw,
	}

	// M4: Governance command group
	govCmd = &cobra.Command{
		Use:   "gov",
		Short: "Governance operations",
	}

	govVoteCmd = &cobra.Command{
		Use:   "vote",
		Short: "Vote on a governance proposal",
		Run:   runGovVote,
	}

	// M5: Mesh/Rosetta API command group
	meshCmd = &cobra.Command{
		Use:   "mesh",
		Short: "Mesh/Rosetta API sidecar management",
		Long: `Manage the Mesh/Rosetta API sidecar for exchange and institution integration.

The sidecar runs as a separate process alongside your node, exposing a
Rosetta-compatible API on port 8080 (by default).

Recommended configuration:
  - OFF for validators (minimize attack surface)
  - ON for RPC/indexer nodes (enable exchange integration)`,
	}

	meshInstallCmd = &cobra.Command{
		Use:   "install",
		Short: "Install the Mesh/Rosetta API binary",
		Run:   runMeshInstall,
	}

	meshEnableCmd = &cobra.Command{
		Use:   "enable",
		Short: "Enable and start the Mesh/Rosetta API service",
		Run:   runMeshEnable,
	}

	meshDisableCmd = &cobra.Command{
		Use:   "disable",
		Short: "Stop and disable the Mesh/Rosetta API service",
		Run:   runMeshDisable,
	}

	meshStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show Mesh/Rosetta API service status",
		Run:   runMeshStatus,
	}

	meshLogsCmd = &cobra.Command{
		Use:   "logs",
		Short: "Tail Mesh/Rosetta API service logs",
		Run:   runMeshLogs,
	}

	// M7: Update command group
	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Commander self-update commands",
		Long: `Manage Commander (monoctl) updates from GitHub Releases.

Check for updates:
  monoctl update check [--json]

Apply updates:
  monoctl update apply [--yes] [--insecure] [--dry-run]

Updates are verified using SHA256 checksums from the release.`,
	}

	updateCheckCmd = &cobra.Command{
		Use:   "check",
		Short: "Check for Commander updates",
		Run:   runUpdateCheck,
	}

	updateApplyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply Commander update",
		Run:   runUpdateApply,
	}

	// Wallet command group
	walletCmd = &cobra.Command{
		Use:   "wallet",
		Short: "Wallet management commands",
		Long: `Generate and manage Monolythium wallets.

Wallets are stored as encrypted keystore v3 JSON files in ~/.mono-commander/wallets/

Generate a new wallet:
  monoctl wallet generate --name my-wallet

List existing wallets:
  monoctl wallet list

Show wallet info:
  monoctl wallet info --file <path>`,
	}

	walletGenerateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate a new wallet keypair",
		Long: `Generate a new secp256k1 keypair and save it as an encrypted keystore.

The keystore is encrypted with your password using scrypt KDF and AES-128-CTR.
By default, the private key is NEVER displayed.

Examples:
  monoctl wallet generate --name my-wallet
  monoctl wallet generate --out /custom/path/wallet.json
  monoctl wallet generate --password-file /path/to/password.txt`,
		Run: runWalletGenerate,
	}

	walletListCmd = &cobra.Command{
		Use:   "list",
		Short: "List wallet keystore files",
		Run:   runWalletList,
	}

	walletInfoCmd = &cobra.Command{
		Use:   "info",
		Short: "Show wallet info from keystore file",
		Long: `Display wallet addresses from a keystore file without decryption.

Shows:
  - EVM address (0x...)
  - Bech32 address (mono1...)
  - Keystore file metadata`,
		Run: runWalletInfo,
	}
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Networks subcommands
	networksCmd.AddCommand(networksListCmd)
	rootCmd.AddCommand(networksCmd)

	// Join command flags
	joinCmd.Flags().String("network", "", "Network to join (Localnet, Sprintnet, Testnet, Mainnet)")
	joinCmd.Flags().String("genesis-url", "", "Genesis file URL (uses network default if not specified)")
	joinCmd.Flags().String("genesis-sha256", "", "Expected SHA256 of genesis file")
	joinCmd.Flags().String("peers-url", "", "Peers registry URL (uses network default if not specified)")
	joinCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	joinCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	joinCmd.MarkFlagRequired("network")
	rootCmd.AddCommand(joinCmd)

	// Peers subcommands
	peersUpdateCmd.Flags().String("network", "", "Network name")
	peersUpdateCmd.Flags().String("peers-url", "", "Peers registry URL")
	peersUpdateCmd.Flags().String("expected-genesis-sha", "", "Expected genesis SHA256")
	peersUpdateCmd.Flags().String("home", "", "Node home directory")
	peersUpdateCmd.Flags().Bool("dry-run", false, "Show what would be done")
	peersUpdateCmd.MarkFlagRequired("network")
	peersCmd.AddCommand(peersUpdateCmd)
	rootCmd.AddCommand(peersCmd)

	// Systemd subcommands
	systemdInstallCmd.Flags().String("network", "", "Network name")
	systemdInstallCmd.Flags().String("home", "", "Node home directory")
	systemdInstallCmd.Flags().String("user", "", "System user to run as")
	systemdInstallCmd.Flags().Bool("cosmovisor", false, "Use Cosmovisor")
	systemdInstallCmd.Flags().Bool("dry-run", false, "Show what would be done")
	systemdInstallCmd.MarkFlagRequired("network")
	systemdInstallCmd.MarkFlagRequired("user")
	systemdCmd.AddCommand(systemdInstallCmd)
	rootCmd.AddCommand(systemdCmd)

	// Status command flags
	statusCmd.Flags().String("network", "Localnet", "Network name")
	statusCmd.Flags().String("home", "", "Node home directory")
	statusCmd.Flags().String("host", "localhost", "RPC host")
	statusCmd.Flags().Bool("remote", false, "Use remote endpoints")
	rootCmd.AddCommand(statusCmd)

	// RPC check command flags
	rpcCheckCmd.Flags().String("network", "Localnet", "Network name")
	rpcCheckCmd.Flags().String("host", "localhost", "RPC host")
	rpcCheckCmd.Flags().Bool("remote", false, "Use remote endpoints")
	rpcCheckCmd.Flags().String("comet-rpc", "", "Override Comet RPC endpoint")
	rpcCheckCmd.Flags().String("cosmos-rest", "", "Override Cosmos REST endpoint")
	rpcCheckCmd.Flags().String("evm-rpc", "", "Override EVM RPC endpoint")
	rpcCmd.AddCommand(rpcCheckCmd)
	rootCmd.AddCommand(rpcCmd)

	// Logs command flags
	logsCmd.Flags().String("network", "Localnet", "Network name")
	logsCmd.Flags().String("home", "", "Node home directory")
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")
	rootCmd.AddCommand(logsCmd)

	// M4: Validator create command
	addTxFlags(validatorCreateCmd)
	validatorCreateCmd.Flags().String("moniker", "", "Validator moniker (required)")
	validatorCreateCmd.Flags().String("identity", "", "Keybase identity (optional)")
	validatorCreateCmd.Flags().String("website", "", "Validator website (optional)")
	validatorCreateCmd.Flags().String("security-contact", "", "Security contact email (optional)")
	validatorCreateCmd.Flags().String("details", "", "Validator details (optional)")
	validatorCreateCmd.Flags().String("commission-rate", "0.10", "Commission rate (e.g., 0.10 for 10%)")
	validatorCreateCmd.Flags().String("commission-max-rate", "0.20", "Maximum commission rate")
	validatorCreateCmd.Flags().String("commission-max-change-rate", "0.01", "Maximum daily commission change")
	validatorCreateCmd.Flags().String("min-self-delegation", "", "Minimum self-delegation in alyth (required, min 100000 LYTH)")
	validatorCreateCmd.Flags().String("amount", "", "Self-bond amount in alyth (required, min 100000 LYTH)")
	validatorCreateCmd.Flags().String("pubkey", "", "Path to consensus pubkey file (optional)")
	validatorCreateCmd.MarkFlagRequired("moniker")
	validatorCreateCmd.MarkFlagRequired("amount")
	validatorCreateCmd.MarkFlagRequired("min-self-delegation")
	validatorCmd.AddCommand(validatorCreateCmd)
	rootCmd.AddCommand(validatorCmd)

	// M4: Stake delegate command
	addTxFlags(stakeDelegateCmd)
	stakeDelegateCmd.Flags().String("to", "", "Validator address (monovaloper1...)")
	stakeDelegateCmd.Flags().String("amount", "", "Amount to delegate in alyth")
	stakeDelegateCmd.MarkFlagRequired("to")
	stakeDelegateCmd.MarkFlagRequired("amount")
	stakeCmd.AddCommand(stakeDelegateCmd)

	// M4: Stake unbond command
	addTxFlags(stakeUnbondCmd)
	stakeUnbondCmd.Flags().String("from-validator", "", "Validator address (monovaloper1...)")
	stakeUnbondCmd.Flags().String("amount", "", "Amount to unbond in alyth")
	stakeUnbondCmd.MarkFlagRequired("from-validator")
	stakeUnbondCmd.MarkFlagRequired("amount")
	stakeCmd.AddCommand(stakeUnbondCmd)

	// M4: Stake redelegate command
	addTxFlags(stakeRedelegateCmd)
	stakeRedelegateCmd.Flags().String("src", "", "Source validator address (monovaloper1...)")
	stakeRedelegateCmd.Flags().String("dst", "", "Destination validator address (monovaloper1...)")
	stakeRedelegateCmd.Flags().String("amount", "", "Amount to redelegate in alyth")
	stakeRedelegateCmd.MarkFlagRequired("src")
	stakeRedelegateCmd.MarkFlagRequired("dst")
	stakeRedelegateCmd.MarkFlagRequired("amount")
	stakeCmd.AddCommand(stakeRedelegateCmd)
	rootCmd.AddCommand(stakeCmd)

	// M4: Rewards withdraw command
	addTxFlags(rewardsWithdrawCmd)
	rewardsWithdrawCmd.Flags().String("validator", "", "Validator address (optional, for specific validator)")
	rewardsWithdrawCmd.Flags().Bool("commission", false, "Also withdraw validator commission")
	rewardsCmd.AddCommand(rewardsWithdrawCmd)
	rootCmd.AddCommand(rewardsCmd)

	// M4: Gov vote command
	addTxFlags(govVoteCmd)
	govVoteCmd.Flags().String("proposal", "", "Proposal ID (required)")
	govVoteCmd.Flags().String("option", "", "Vote option: yes, no, abstain, no_with_veto (required)")
	govVoteCmd.MarkFlagRequired("proposal")
	govVoteCmd.MarkFlagRequired("option")
	govCmd.AddCommand(govVoteCmd)
	rootCmd.AddCommand(govCmd)

	// M5: Mesh/Rosetta API commands
	// mesh install
	meshInstallCmd.Flags().String("network", "Localnet", "Network name (Localnet, Sprintnet, Testnet, Mainnet)")
	meshInstallCmd.Flags().String("home", "", "Node home directory (default: user home)")
	meshInstallCmd.Flags().String("version", "", "Version to install")
	meshInstallCmd.Flags().String("url", "", "Download URL for the binary")
	meshInstallCmd.Flags().String("sha256", "", "Expected SHA256 checksum of the binary")
	meshInstallCmd.Flags().Bool("insecure", false, "Skip checksum verification (not recommended)")
	meshInstallCmd.Flags().Bool("system", false, "Install to /usr/local/bin (requires sudo)")
	meshInstallCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	meshCmd.AddCommand(meshInstallCmd)

	// mesh enable
	meshEnableCmd.Flags().String("network", "Localnet", "Network name")
	meshEnableCmd.Flags().String("home", "", "Node home directory")
	meshEnableCmd.Flags().String("listen", "", "Listen address (default: 0.0.0.0:8080)")
	meshEnableCmd.Flags().String("node-rpc", "", "Node RPC URL (default: http://localhost:26657)")
	meshEnableCmd.Flags().String("node-grpc", "", "Node gRPC address (default: localhost:9090)")
	meshEnableCmd.Flags().String("user", "", "System user to run as (default: current user)")
	meshEnableCmd.Flags().Bool("dry-run", false, "Show what would be done")
	meshCmd.AddCommand(meshEnableCmd)

	// mesh disable
	meshDisableCmd.Flags().String("network", "Localnet", "Network name")
	meshDisableCmd.Flags().String("home", "", "Node home directory")
	meshDisableCmd.Flags().Bool("dry-run", false, "Show what would be done")
	meshCmd.AddCommand(meshDisableCmd)

	// mesh status
	meshStatusCmd.Flags().String("network", "Localnet", "Network name")
	meshStatusCmd.Flags().String("home", "", "Node home directory")
	meshCmd.AddCommand(meshStatusCmd)

	// mesh logs
	meshLogsCmd.Flags().String("network", "Localnet", "Network name")
	meshLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	meshLogsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")
	meshCmd.AddCommand(meshLogsCmd)

	rootCmd.AddCommand(meshCmd)

	// M7: Update commands
	updateCmd.AddCommand(updateCheckCmd)

	updateApplyCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	updateApplyCmd.Flags().Bool("insecure", false, "Skip checksum verification (not recommended)")
	updateApplyCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	updateCmd.AddCommand(updateApplyCmd)

	rootCmd.AddCommand(updateCmd)

	// Wallet commands
	walletGenerateCmd.Flags().String("name", "", "Wallet name (optional, used in filename)")
	walletGenerateCmd.Flags().String("out", "", "Output path for keystore file (default: ~/.mono-commander/wallets/)")
	walletGenerateCmd.Flags().String("password-file", "", "Path to file containing password (alternative to interactive prompt)")
	walletGenerateCmd.Flags().Bool("show-private-key", false, "Show private key after generation (DANGEROUS)")
	walletGenerateCmd.Flags().Bool("insecure-show", false, "Required with --show-private-key to confirm understanding")
	walletCmd.AddCommand(walletGenerateCmd)

	walletListCmd.Flags().String("dir", "", "Directory to list (default: ~/.mono-commander/wallets/)")
	walletCmd.AddCommand(walletListCmd)

	walletInfoCmd.Flags().String("file", "", "Path to keystore file (required)")
	walletInfoCmd.MarkFlagRequired("file")
	walletCmd.AddCommand(walletInfoCmd)

	rootCmd.AddCommand(walletCmd)
}

// addTxFlags adds common transaction flags to a command
func addTxFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&txNetwork, "network", "Localnet", "Network name (Localnet, Sprintnet, Testnet, Mainnet)")
	cmd.Flags().StringVar(&txHome, "home", "", "Node home directory (default: ~/.monod)")
	cmd.Flags().StringVar(&txFrom, "from", "", "Key name or address to sign with (required)")
	cmd.Flags().StringVar(&txFees, "fees", "", "Transaction fees in alyth (e.g., 10000alyth)")
	cmd.Flags().StringVar(&txGasPrices, "gas-prices", "", "Gas prices in alyth (e.g., 0.025alyth)")
	cmd.Flags().StringVar(&txGas, "gas", "auto", "Gas limit or 'auto'")
	cmd.Flags().StringVar(&txNode, "node", "", "RPC node URL (default: localhost:26657)")
	cmd.Flags().StringVar(&txChainID, "chain-id", "", "Chain ID override (uses network default if empty)")
	cmd.Flags().BoolVar(&txDryRun, "dry-run", true, "Only show command, do not execute")
	cmd.Flags().BoolVar(&txExecute, "execute", false, "Execute the transaction (overrides dry-run)")
	cmd.MarkFlagRequired("from")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runNetworksList(cmd *cobra.Command, args []string) {
	networks := core.ListNetworks()

	if jsonOutput {
		type networkJSON struct {
			Name       string `json:"name"`
			ChainID    string `json:"chain_id"`
			EVMChainID uint64 `json:"evm_chain_id"`
			EVMHex     string `json:"evm_chain_id_hex"`
		}

		out := make([]networkJSON, len(networks))
		for i, n := range networks {
			out[i] = networkJSON{
				Name:       string(n.Name),
				ChainID:    n.ChainID,
				EVMChainID: n.EVMChainID,
				EVMHex:     n.EVMChainIDHex(),
			}
		}

		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("Supported Networks:")
	fmt.Println()
	fmt.Printf("%-12s %-15s %-10s %s\n", "NAME", "CHAIN ID", "EVM ID", "EVM HEX")
	fmt.Println(strings.Repeat("-", 50))
	for _, n := range networks {
		fmt.Printf("%-12s %-15s %-10d %s\n", n.Name, n.ChainID, n.EVMChainID, n.EVMChainIDHex())
	}
}

func runJoin(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	genesisURL, _ := cmd.Flags().GetString("genesis-url")
	genesisSHA, _ := cmd.Flags().GetString("genesis-sha256")
	peersURL, _ := cmd.Flags().GetString("peers-url")
	home, _ := cmd.Flags().GetString("home")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Parse network name
	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Default home
	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = homeDir + "/.monod"
	}

	// Setup logger
	var logger *slog.Logger
	if verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	opts := core.JoinOptions{
		Network:    network,
		Home:       home,
		GenesisURL: genesisURL,
		GenesisSHA: genesisSHA,
		PeersURL:   peersURL,
		DryRun:     dryRun,
		Logger:     logger,
	}

	fetcher := net.NewHTTPFetcher()
	result, err := core.Join(opts, fetcher)

	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		if err != nil {
			os.Exit(1)
		}
		return
	}

	// Print steps
	fmt.Printf("Join Network: %s\n", network)
	if dryRun {
		fmt.Println("(DRY RUN - no changes will be made)")
	}
	fmt.Println()

	for _, step := range result.Steps {
		status := "[ ]"
		switch step.Status {
		case "success":
			status = "[+]"
		case "failed":
			status = "[X]"
		case "skipped":
			status = "[-]"
		}
		msg := step.Name
		if step.Message != "" {
			msg += ": " + step.Message
		}
		fmt.Printf("%s %s\n", status, msg)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Genesis written to: %s\n", result.GenesisPath)
	fmt.Printf("Chain ID: %s\n", result.ChainID)

	if dryRun {
		fmt.Println("\nConfig patch that would be applied:")
		fmt.Println(result.ConfigPatch)
	}
}

func runPeersUpdate(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	peersURL, _ := cmd.Flags().GetString("peers-url")
	expectedSHA, _ := cmd.Flags().GetString("expected-genesis-sha")
	home, _ := cmd.Flags().GetString("home")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	netCfg, err := core.GetNetwork(network)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if peersURL == "" {
		peersURL = netCfg.PeersURL
	}

	if peersURL == "" {
		fmt.Fprintf(os.Stderr, "Error: peers URL required\n")
		os.Exit(1)
	}

	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = homeDir + "/.monod"
	}

	fetcher := net.NewHTTPFetcher()
	data, err := fetcher.Fetch(peersURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching peers: %v\n", err)
		os.Exit(1)
	}

	reg, err := core.ParsePeersRegistry(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing peers: %v\n", err)
		os.Exit(1)
	}

	if err := core.ValidatePeersRegistry(reg, netCfg.ChainID, expectedSHA); err != nil {
		fmt.Fprintf(os.Stderr, "Error validating peers: %v\n", err)
		os.Exit(1)
	}

	peers := core.MergePeers(reg.Peers, reg.PersistentPeers)
	patch := core.GenerateConfigPatch(netCfg, peers)
	patchPath, content, err := core.WriteConfigPatch(home, patch, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		out := map[string]interface{}{
			"peers_count": len(peers),
			"patch_path":  patchPath,
			"dry_run":     dryRun,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Peers updated: %d peers\n", len(peers))
	if dryRun {
		fmt.Println("\nConfig patch (dry-run):")
		fmt.Println(content)
	} else {
		fmt.Printf("Config patch written to: %s\n", patchPath)
	}
}

func runSystemdInstall(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")
	user, _ := cmd.Flags().GetString("user")
	useCosmovisor, _ := cmd.Flags().GetBool("cosmovisor")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = homeDir + "/.monod"
	}

	cfg := oshelpers.DefaultSystemdConfig(string(network), user, home)
	cfg.UseCosmovisor = useCosmovisor

	unitPath, content, err := oshelpers.WriteSystemdUnit(cfg, dryRun)
	if err != nil && !dryRun {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		out := map[string]interface{}{
			"unit_path": unitPath,
			"dry_run":   dryRun,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	if dryRun {
		fmt.Printf("Would write to: %s\n\n", unitPath)
		fmt.Println("--- Unit file content ---")
		fmt.Println(content)
		fmt.Println("--- End ---")
	} else {
		fmt.Println(oshelpers.SystemdInstructions(unitPath))
	}
}

func runStatus(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")
	host, _ := cmd.Flags().GetString("host")
	useRemote, _ := cmd.Flags().GetBool("remote")

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = homeDir + "/.monod"
	}

	endpoints := resolveEndpoints(string(network), host, useRemote, "", "", "")

	opts := core.StatusOptions{
		Network:   network,
		Endpoints: endpoints,
	}

	status, err := core.GetNodeStatus(opts)
	if err != nil {
		if jsonOutput {
			out := map[string]interface{}{
				"error": err.Error(),
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	// Add service status on Linux
	status.ServiceStatus = logs.GetSystemdServiceStatus(string(network))

	if jsonOutput {
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	fmt.Println("Node Status")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Chain ID:      %s\n", status.ChainID)
	fmt.Printf("Moniker:       %s\n", status.Moniker)
	fmt.Printf("Version:       %s\n", status.NodeVersion)
	fmt.Printf("Latest Height: %d\n", status.LatestHeight)
	fmt.Printf("Catching Up:   %t\n", status.CatchingUp)
	fmt.Printf("Peers:         %d\n", status.PeersCount)
	if status.ServiceStatus != "" {
		fmt.Printf("Service:       %s\n", status.ServiceStatus)
	}
}

func runRPCCheck(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	host, _ := cmd.Flags().GetString("host")
	useRemote, _ := cmd.Flags().GetBool("remote")
	cometRPC, _ := cmd.Flags().GetString("comet-rpc")
	cosmosREST, _ := cmd.Flags().GetString("cosmos-rest")
	evmRPC, _ := cmd.Flags().GetString("evm-rpc")

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	endpoints := resolveEndpoints(string(network), host, useRemote, cometRPC, cosmosREST, evmRPC)

	results := core.CheckRPC(network, endpoints)

	if jsonOutput {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		if !results.AllPass {
			os.Exit(1)
		}
		return
	}

	// Human-readable output
	fmt.Printf("RPC Health Check - %s\n", network)
	fmt.Println(strings.Repeat("-", 50))

	for _, r := range results.Results {
		status := "[PASS]"
		if r.Status == "FAIL" {
			status = "[FAIL]"
		}
		fmt.Printf("%s %s (%s)\n", status, r.Type, r.Endpoint)
		if r.Details != "" {
			fmt.Printf("       %s\n", r.Details)
		}
		if r.Message != "" {
			fmt.Printf("       Error: %s\n", r.Message)
		}
	}

	fmt.Println()
	if results.AllPass {
		fmt.Println("All RPC endpoints healthy.")
	} else {
		fmt.Println("Some RPC endpoints failed.")
		os.Exit(1)
	}
}

// resolveEndpoints resolves RPC endpoints based on network and options.
func resolveEndpoints(network, host string, useRemote bool, cometRPC, cosmosREST, evmRPC string) core.Endpoints {
	var endpoints core.Endpoints

	if useRemote {
		// Remote endpoints for public networks
		switch network {
		case "Sprintnet":
			endpoints = core.Endpoints{
				CometRPC:   "https://rpc.sprintnet.monolythium.com",
				CosmosREST: "https://api.sprintnet.monolythium.com",
				EVMRPC:     "https://evm.sprintnet.monolythium.com",
			}
		case "Testnet":
			endpoints = core.Endpoints{
				CometRPC:   "https://rpc.testnet.monolythium.com",
				CosmosREST: "https://api.testnet.monolythium.com",
				EVMRPC:     "https://evm.testnet.monolythium.com",
			}
		case "Mainnet":
			endpoints = core.Endpoints{
				CometRPC:   "https://rpc.monolythium.com",
				CosmosREST: "https://api.monolythium.com",
				EVMRPC:     "https://evm.monolythium.com",
			}
		default:
			// Localnet - use local endpoints
			if host == "" {
				host = "localhost"
			}
			endpoints = core.Endpoints{
				CometRPC:   fmt.Sprintf("http://%s:26657", host),
				CosmosREST: fmt.Sprintf("http://%s:1317", host),
				EVMRPC:     fmt.Sprintf("http://%s:8545", host),
			}
		}
	} else {
		// Local endpoints
		if host == "" {
			host = "localhost"
		}
		endpoints = core.Endpoints{
			CometRPC:   fmt.Sprintf("http://%s:26657", host),
			CosmosREST: fmt.Sprintf("http://%s:1317", host),
			EVMRPC:     fmt.Sprintf("http://%s:8545", host),
		}
	}

	// Apply overrides
	if cometRPC != "" {
		endpoints.CometRPC = cometRPC
	}
	if cosmosREST != "" {
		endpoints.CosmosREST = cosmosREST
	}
	if evmRPC != "" {
		endpoints.EVMRPC = evmRPC
	}

	return endpoints
}

func runLogs(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")
	follow, _ := cmd.Flags().GetBool("follow")
	lines, _ := cmd.Flags().GetInt("lines")

	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = homeDir + "/.monod"
	}

	source, err := logs.GetLogSource(networkStr, home, follow, lines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer source.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	linesCh, err := source.Lines(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for line := range linesCh {
		fmt.Println(line)
	}
}

// =============================================================================
// M4: Validator Action Commands
// =============================================================================

// getTxOptions builds ValidatorActionOptions from command flags
func getTxOptions(cmd *cobra.Command) core.ValidatorActionOptions {
	// Read flags fresh from the command to avoid stale globals
	networkStr, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")
	from, _ := cmd.Flags().GetString("from")
	fees, _ := cmd.Flags().GetString("fees")
	gasPrices, _ := cmd.Flags().GetString("gas-prices")
	gas, _ := cmd.Flags().GetString("gas")
	node, _ := cmd.Flags().GetString("node")
	chainID, _ := cmd.Flags().GetString("chain-id")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	execute, _ := cmd.Flags().GetBool("execute")

	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = homeDir + "/.monod"
	}

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		network = core.NetworkLocalnet
	}

	var logger *slog.Logger
	if verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	return core.ValidatorActionOptions{
		Network:   network,
		Home:      home,
		From:      from,
		Fees:      fees,
		GasPrices: gasPrices,
		Gas:       gas,
		Node:      node,
		ChainID:   chainID,
		DryRun:    dryRun && !execute, // execute overrides dry-run
		Execute:   execute,
		Logger:    logger,
	}
}

// printActionResult prints the result of a validator action
func printActionResult(result *core.ValidatorActionResult) {
	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Print description
	fmt.Printf("%s\n", result.Description)
	fmt.Println(strings.Repeat("-", 60))

	// Print steps
	for _, step := range result.Steps {
		status := "[ ]"
		switch step.Status {
		case "success":
			status = "[+]"
		case "failed":
			status = "[X]"
		case "skipped":
			status = "[-]"
		}
		msg := step.Name
		if step.Message != "" {
			msg += ": " + step.Message
		}
		fmt.Printf("%s %s\n", status, msg)
	}

	// Print warnings
	for _, warn := range result.Warnings {
		fmt.Printf("\nWARNING: %s\n", warn)
	}

	// Print command preview
	if result.Command != nil && !result.Executed {
		fmt.Println("\nGenerated command:")
		fmt.Printf("  %s\n", result.Command.String())

		if result.Command.RequiresMultiMsg && len(result.Command.MultiMsgCommands) > 0 {
			fmt.Println("\nMulti-message transaction components:")
			for i, subCmd := range result.Command.MultiMsgCommands {
				fmt.Printf("  [%d] %s\n", i+1, subCmd.Description)
				fmt.Printf("      %s\n", subCmd.String())
			}
		}
	}

	// Print execution result
	if result.Executed {
		if result.Success {
			fmt.Println("\nTransaction submitted successfully!")
			if result.TxHash != "" {
				fmt.Printf("  TxHash: %s\n", result.TxHash)
			}
			if result.Height > 0 {
				fmt.Printf("  Height: %d\n", result.Height)
			}
		} else {
			fmt.Println("\nTransaction failed!")
		}
	}
}

func runValidatorCreate(cmd *cobra.Command, args []string) {
	opts := getTxOptions(cmd)

	// Get validator-specific flags
	moniker, _ := cmd.Flags().GetString("moniker")
	identity, _ := cmd.Flags().GetString("identity")
	website, _ := cmd.Flags().GetString("website")
	securityContact, _ := cmd.Flags().GetString("security-contact")
	details, _ := cmd.Flags().GetString("details")
	commissionRate, _ := cmd.Flags().GetString("commission-rate")
	commissionMaxRate, _ := cmd.Flags().GetString("commission-max-rate")
	commissionMaxChange, _ := cmd.Flags().GetString("commission-max-change-rate")
	minSelfDelegation, _ := cmd.Flags().GetString("min-self-delegation")
	amount, _ := cmd.Flags().GetString("amount")
	pubkeyPath, _ := cmd.Flags().GetString("pubkey")

	params := core.CreateValidatorParams{
		Moniker:             moniker,
		Identity:            identity,
		Website:             website,
		SecurityContact:     securityContact,
		Details:             details,
		CommissionRate:      commissionRate,
		CommissionMaxRate:   commissionMaxRate,
		CommissionMaxChange: commissionMaxChange,
		MinSelfDelegation:   minSelfDelegation,
		Amount:              amount,
		PubKeyPath:          pubkeyPath,
	}

	ctx := context.Background()
	result, err := core.CreateValidatorAction(ctx, opts, params)
	if err != nil && result == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	printActionResult(result)

	if result != nil && !result.Success {
		os.Exit(1)
	}
}

func runStakeDelegate(cmd *cobra.Command, args []string) {
	opts := getTxOptions(cmd)

	validatorAddr, _ := cmd.Flags().GetString("to")
	amount, _ := cmd.Flags().GetString("amount")

	params := core.DelegateParams{
		ValidatorAddr: validatorAddr,
		Amount:        amount,
	}

	ctx := context.Background()
	result, err := core.DelegateAction(ctx, opts, params)
	if err != nil && result == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	printActionResult(result)

	if result != nil && !result.Success {
		os.Exit(1)
	}
}

func runStakeUnbond(cmd *cobra.Command, args []string) {
	opts := getTxOptions(cmd)

	validatorAddr, _ := cmd.Flags().GetString("from-validator")
	amount, _ := cmd.Flags().GetString("amount")

	params := core.UnbondParams{
		ValidatorAddr: validatorAddr,
		Amount:        amount,
	}

	ctx := context.Background()
	result, err := core.UnbondAction(ctx, opts, params)
	if err != nil && result == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	printActionResult(result)

	if result != nil && !result.Success {
		os.Exit(1)
	}
}

func runStakeRedelegate(cmd *cobra.Command, args []string) {
	opts := getTxOptions(cmd)

	srcValidator, _ := cmd.Flags().GetString("src")
	dstValidator, _ := cmd.Flags().GetString("dst")
	amount, _ := cmd.Flags().GetString("amount")

	params := core.RedelegateParams{
		SrcValidatorAddr: srcValidator,
		DstValidatorAddr: dstValidator,
		Amount:           amount,
	}

	ctx := context.Background()
	result, err := core.RedelegateAction(ctx, opts, params)
	if err != nil && result == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	printActionResult(result)

	if result != nil && !result.Success {
		os.Exit(1)
	}
}

func runRewardsWithdraw(cmd *cobra.Command, args []string) {
	opts := getTxOptions(cmd)

	validatorAddr, _ := cmd.Flags().GetString("validator")
	commission, _ := cmd.Flags().GetBool("commission")

	params := core.WithdrawRewardsParams{
		ValidatorAddr: validatorAddr,
		Commission:    commission,
	}

	ctx := context.Background()
	result, err := core.WithdrawRewardsAction(ctx, opts, params)
	if err != nil && result == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	printActionResult(result)

	if result != nil && !result.Success {
		os.Exit(1)
	}
}

func runGovVote(cmd *cobra.Command, args []string) {
	opts := getTxOptions(cmd)

	proposalID, _ := cmd.Flags().GetString("proposal")
	optionStr, _ := cmd.Flags().GetString("option")

	// Validate vote option
	option, err := core.ValidateVoteOption(optionStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	params := core.VoteParams{
		ProposalID: proposalID,
		Option:     option,
	}

	ctx := context.Background()
	result, err := core.VoteAction(ctx, opts, params)
	if err != nil && result == nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	printActionResult(result)

	if result != nil && !result.Success {
		os.Exit(1)
	}
}

// =============================================================================
// M5: Mesh/Rosetta API Commands
// =============================================================================

func runMeshInstall(cmd *cobra.Command, args []string) {
	url, _ := cmd.Flags().GetString("url")
	sha256sum, _ := cmd.Flags().GetString("sha256")
	version, _ := cmd.Flags().GetString("version")
	insecure, _ := cmd.Flags().GetBool("insecure")
	useSystem, _ := cmd.Flags().GetBool("system")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	opts := mesh.InstallOptions{
		URL:           url,
		SHA256:        sha256sum,
		Version:       version,
		UseSystemPath: useSystem,
		Insecure:      insecure,
		DryRun:        dryRun,
	}

	result := mesh.Install(opts)

	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		if !result.Success {
			os.Exit(1)
		}
		return
	}

	// Human-readable output
	fmt.Println("Mesh/Rosetta API Binary Installation")
	fmt.Println(strings.Repeat("-", 50))

	if dryRun {
		fmt.Println("(DRY RUN - no changes will be made)")
		fmt.Println()
	}

	for _, step := range result.Steps {
		status := "[ ]"
		switch step.Status {
		case "success":
			status = "[+]"
		case "failed":
			status = "[X]"
		case "skipped":
			status = "[-]"
		}
		msg := step.Name
		if step.Message != "" {
			msg += ": " + step.Message
		}
		fmt.Printf("%s %s\n", status, msg)
	}

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", result.Error)
		os.Exit(1)
	}

	if result.Success && !dryRun {
		fmt.Printf("\nBinary installed to: %s\n", result.InstallPath)
		if result.SHA256 != "" {
			fmt.Printf("SHA256: %s\n", result.SHA256)
		}
	}
}

func runMeshEnable(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")
	listen, _ := cmd.Flags().GetString("listen")
	nodeRPC, _ := cmd.Flags().GetString("node-rpc")
	nodeGRPC, _ := cmd.Flags().GetString("node-grpc")
	user, _ := cmd.Flags().GetString("user")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = homeDir
	}

	if user == "" {
		user = os.Getenv("USER")
		if user == "" {
			user = "monod"
		}
	}

	// Create/load config
	var cfg *mesh.Config
	if mesh.ConfigExists(home, network) {
		cfg, err = mesh.LoadConfig(home, network)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
	} else {
		cfg = mesh.DefaultConfig(network)
	}

	// Apply overrides
	cfg.Merge(mesh.MergeOptions{
		ListenAddress:   listen,
		NodeRPCURL:      nodeRPC,
		NodeGRPCAddress: nodeGRPC,
	})

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Save config
	configContent, err := mesh.SaveConfig(home, network, cfg, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	// Generate systemd unit
	systemdCfg := mesh.DefaultSystemdConfig(string(network), user, home, network)
	unitPath, unitContent, err := mesh.WriteSystemdUnit(systemdCfg, dryRun)
	if err != nil && !dryRun {
		fmt.Fprintf(os.Stderr, "Error writing systemd unit: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		out := map[string]interface{}{
			"config_path":  mesh.ConfigPath(home, network),
			"unit_path":    unitPath,
			"unit_name":    mesh.UnitName(string(network)),
			"dry_run":      dryRun,
			"config":       cfg,
			"unit_content": unitContent,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	fmt.Println("Mesh/Rosetta API Service Enable")
	fmt.Println(strings.Repeat("-", 50))

	if dryRun {
		fmt.Println("(DRY RUN - no changes will be made)")
		fmt.Println()

		fmt.Println("Config file would be written to:")
		fmt.Printf("  %s\n\n", mesh.ConfigPath(home, network))
		fmt.Println("Config content:")
		fmt.Println(configContent)
		fmt.Println()

		fmt.Println("Systemd unit file would be written to:")
		fmt.Printf("  %s\n\n", unitPath)
		fmt.Println("Unit content:")
		fmt.Println(unitContent)
		return
	}

	// Enable and start service
	result, err := mesh.EnableService(string(network), false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enabling service: %v\n", err)
		fmt.Println()
		fmt.Println("You may need to run the following commands manually:")
		fmt.Println(mesh.SystemdInstructions(unitPath, mesh.UnitName(string(network))))
		os.Exit(1)
	}

	fmt.Printf("[+] Config written to: %s\n", mesh.ConfigPath(home, network))
	fmt.Printf("[+] Unit file written to: %s\n", unitPath)
	if result.Enabled {
		fmt.Printf("[+] Service enabled: %s\n", result.UnitName)
	}
	if result.Started {
		fmt.Printf("[+] Service started: %s\n", result.UnitName)
	}

	fmt.Println()
	fmt.Printf("Mesh/Rosetta API is now running at: %s\n", cfg.ListenAddress)
}

func runMeshDisable(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput && dryRun {
		out := map[string]interface{}{
			"unit_name": mesh.UnitName(string(network)),
			"dry_run":   true,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	result, err := mesh.DisableService(string(network), dryRun)

	if jsonOutput {
		out := map[string]interface{}{
			"unit_name": result.UnitName,
			"stopped":   result.Stopped,
			"disabled":  result.Disabled,
			"dry_run":   dryRun,
		}
		if result.Error != nil {
			out["error"] = result.Error.Error()
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		if result.Error != nil {
			os.Exit(1)
		}
		return
	}

	fmt.Println("Mesh/Rosetta API Service Disable")
	fmt.Println(strings.Repeat("-", 50))

	if dryRun {
		fmt.Println("(DRY RUN)")
		fmt.Printf("Would stop and disable: %s\n", mesh.UnitName(string(network)))
		return
	}

	if result.Stopped {
		fmt.Printf("[+] Service stopped: %s\n", result.UnitName)
	}
	if result.Disabled {
		fmt.Printf("[+] Service disabled: %s\n", result.UnitName)
	}

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: %v\n", result.Error)
	}
}

func runMeshStatus(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = homeDir
	}

	ctx := context.Background()
	result := mesh.FullCheck(ctx, string(network), home, network)

	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	fmt.Printf("Mesh/Rosetta API Status - %s\n", network)
	fmt.Println(strings.Repeat("-", 50))

	// Binary status
	binaryStatus := "NOT INSTALLED"
	if result.BinaryExists {
		binaryStatus = "INSTALLED"
	}
	fmt.Printf("Binary:        %s\n", binaryStatus)

	// Config status
	configStatus := "NOT CONFIGURED"
	if result.ConfigExists {
		configStatus = "CONFIGURED"
	}
	fmt.Printf("Config:        %s\n", configStatus)

	// Systemd status
	if result.SystemdStatus != nil {
		fmt.Printf("Service:       %s (%s)\n", result.SystemdStatus.ActiveState, result.SystemdStatus.SubState)
		if result.SystemdStatus.MainPID > 0 {
			fmt.Printf("PID:           %d\n", result.SystemdStatus.MainPID)
		}
	} else {
		fmt.Printf("Service:       (systemd not available)\n")
	}

	// Health check
	if result.ServiceHealth != nil {
		healthStatus := "UNHEALTHY"
		if result.ServiceHealth.Healthy {
			healthStatus = "HEALTHY"
		}
		fmt.Printf("Health:        %s (via %s, %dms)\n",
			healthStatus,
			result.ServiceHealth.Method,
			result.ServiceHealth.ResponseTime)
		if result.ServiceHealth.Error != "" {
			fmt.Printf("Error:         %s\n", result.ServiceHealth.Error)
		}
		fmt.Printf("Listen:        %s\n", result.ServiceHealth.ListenAddress)
	}
}

func runMeshLogs(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	follow, _ := cmd.Flags().GetBool("follow")
	lines, _ := cmd.Flags().GetInt("lines")

	network, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	opts := mesh.LogsOptions{
		Network: string(network),
		Follow:  follow,
		Lines:   lines,
	}

	source, err := mesh.GetLogSource(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer source.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	linesCh, err := source.Lines(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for line := range linesCh {
		fmt.Println(line)
	}
}

// =============================================================================
// M7: Update Commands
// =============================================================================

func runUpdateCheck(cmd *cobra.Command, args []string) {
	client := update.NewClient()
	result, err := client.Check(tui.Version)

	if err != nil {
		if jsonOutput {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"error": err.Error(),
			}, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	fmt.Println("Commander Update Check")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Current version:  %s\n", result.CurrentVersion)
	fmt.Printf("Latest version:   %s\n", result.LatestVersion)
	fmt.Printf("Published:        %s\n", result.PublishedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Release URL:      %s\n", result.ReleaseURL)
	fmt.Println()

	switch result.Status {
	case "up-to-date":
		fmt.Println("Status: ✓ Up to date")
	case "update-available":
		fmt.Println("Status: ⚠ Update available")
		if result.DownloadURL != "" {
			fmt.Printf("\nDownload: %s\n", result.DownloadURL)
		}
		fmt.Println("\nRun 'monoctl update apply' to update.")
	default:
		fmt.Println("Status: Unknown")
		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
		}
	}
}

func runUpdateApply(cmd *cobra.Command, args []string) {
	yes, _ := cmd.Flags().GetBool("yes")
	insecure, _ := cmd.Flags().GetBool("insecure")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	client := update.NewClient()

	// First check for updates
	checkResult, err := client.Check(tui.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}

	if !checkResult.UpdateAvailable {
		fmt.Println("Already up to date.")
		return
	}

	// Show what we're about to do
	fmt.Printf("Update available: %s → %s\n", checkResult.CurrentVersion, checkResult.LatestVersion)

	if checkResult.DownloadURL == "" {
		fmt.Fprintf(os.Stderr, "Error: No matching asset for your OS/architecture\n")
		fmt.Fprintf(os.Stderr, "Download manually from: %s\n", checkResult.ReleaseURL)
		os.Exit(1)
	}

	if dryRun {
		fmt.Println("(DRY RUN - no changes will be made)")
	}

	// Confirm unless --yes
	if !yes && !dryRun {
		fmt.Print("\nProceed with update? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Update cancelled.")
			return
		}
	}

	opts := update.ApplyOptions{
		CurrentVersion: tui.Version,
		Yes:            yes,
		Insecure:       insecure,
		DryRun:         dryRun,
		OnProgress: func(step, message string) {
			if verbose {
				fmt.Printf("[%s] %s\n", step, message)
			}
		},
	}

	result, err := client.Apply(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		if !result.Success {
			os.Exit(1)
		}
		return
	}

	// Human-readable output
	fmt.Println()
	fmt.Println("Commander Update")
	fmt.Println(strings.Repeat("-", 50))

	for _, step := range result.Steps {
		status := "[ ]"
		switch step.Status {
		case "success":
			status = "[+]"
		case "failed":
			status = "[X]"
		case "skipped":
			status = "[-]"
		}
		msg := step.Name
		if step.Message != "" {
			msg += ": " + step.Message
		}
		fmt.Printf("%s %s\n", status, msg)
	}

	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "\nError: %s\n", result.Error)
		if result.NeedsSudo {
			fmt.Println("\nTo complete the update manually:")
			fmt.Printf("  1. Download: %s\n", checkResult.DownloadURL)
			fmt.Printf("  2. Verify checksum (if available)\n")
			fmt.Printf("  3. sudo mv <downloaded-binary> %s\n", result.PreviousPath)
		}
		os.Exit(1)
	}

	if result.Success {
		fmt.Println()
		if dryRun {
			fmt.Printf("Would update: %s → %s\n", result.OldVersion, result.NewVersion)
		} else {
			fmt.Printf("Update successful: %s → %s\n", result.OldVersion, result.NewVersion)
			if result.BackupPath != "" {
				fmt.Printf("Backup stored at: %s\n", result.BackupPath)
			}
			if result.ChecksumVerify {
				fmt.Println("Checksum verified ✓")
			}
			fmt.Println("\nPlease restart monoctl to use the new version.")
		}
	}
}

// =============================================================================
// Wallet Commands
// =============================================================================

func runWalletGenerate(cmd *cobra.Command, args []string) {
	name, _ := cmd.Flags().GetString("name")
	outPath, _ := cmd.Flags().GetString("out")
	passwordFile, _ := cmd.Flags().GetString("password-file")
	showPrivateKey, _ := cmd.Flags().GetBool("show-private-key")
	insecureShow, _ := cmd.Flags().GetBool("insecure-show")

	// Validate --show-private-key requires --insecure-show
	if showPrivateKey && !insecureShow {
		fmt.Fprintln(os.Stderr, "ERROR: --show-private-key requires --insecure-show=true")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "This is a dangerous operation. If you proceed, your private key")
		fmt.Fprintln(os.Stderr, "will be displayed on screen. Never share your private key with anyone.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "To proceed, run:")
		fmt.Fprintln(os.Stderr, "  monoctl wallet generate --show-private-key --insecure-show=true")
		os.Exit(1)
	}

	// Get password
	var password string
	var err error

	if passwordFile != "" {
		// Read password from file
		data, err := os.ReadFile(passwordFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password file: %v\n", err)
			os.Exit(1)
		}
		password = strings.TrimSpace(string(data))
	} else {
		// Interactive password prompt
		password, err = promptPassword("Enter password for keystore: ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}

		// Confirm password
		confirm, err := promptPassword("Confirm password: ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading password: %v\n", err)
			os.Exit(1)
		}

		if password != confirm {
			fmt.Fprintln(os.Stderr, "Error: passwords do not match")
			os.Exit(1)
		}
	}

	if len(password) < 8 {
		fmt.Fprintln(os.Stderr, "Error: password must be at least 8 characters")
		os.Exit(1)
	}

	// Generate keypair
	kp, err := walletgen.GenerateKeypair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating keypair: %v\n", err)
		os.Exit(1)
	}

	// Create keystore
	ks, err := walletgen.CreateKeystore(kp, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating keystore: %v\n", err)
		os.Exit(1)
	}

	// Determine output path
	if outPath == "" {
		walletDir, err := walletgen.GetDefaultWalletDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting wallet directory: %v\n", err)
			os.Exit(1)
		}
		filename := walletgen.GenerateKeystoreFilename(name, kp.EVMAddress())
		outPath = filepath.Join(walletDir, filename)
	}

	// Save keystore
	if err := walletgen.SaveKeystore(ks, outPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving keystore: %v\n", err)
		os.Exit(1)
	}

	// Get addresses
	evmAddr := kp.EVMAddress()
	bech32Addr, _ := kp.Bech32Address()

	if jsonOutput {
		out := map[string]interface{}{
			"keystore_path":  outPath,
			"evm_address":    evmAddr,
			"bech32_address": bech32Addr,
		}
		if showPrivateKey && insecureShow {
			out["private_key"] = kp.PrivateKeyHex()
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	fmt.Println()
	fmt.Println("Wallet Generated Successfully")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Keystore:       %s\n", outPath)
	fmt.Printf("EVM Address:    %s\n", evmAddr)
	fmt.Printf("Bech32 Address: %s\n", bech32Addr)

	if showPrivateKey && insecureShow {
		fmt.Println()
		fmt.Println(strings.Repeat("!", 60))
		fmt.Println("!!! WARNING: PRIVATE KEY BELOW - NEVER SHARE THIS !!!")
		fmt.Println(strings.Repeat("!", 60))
		fmt.Printf("Private Key:    %s\n", kp.PrivateKeyHex())
		fmt.Println(strings.Repeat("!", 60))
		fmt.Println()
		fmt.Println("The private key above gives FULL CONTROL over this wallet.")
		fmt.Println("Store it securely and NEVER share it with anyone.")
	}

	fmt.Println()
	fmt.Println("Keep your password safe - it is required to use this wallet.")
}

func runWalletList(cmd *cobra.Command, args []string) {
	dir, _ := cmd.Flags().GetString("dir")

	if dir == "" {
		var err error
		dir, err = walletgen.GetDefaultWalletDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting wallet directory: %v\n", err)
			os.Exit(1)
		}
	}

	infos, err := walletgen.ListKeystores(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing keystores: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(infos, "", "  ")
		fmt.Println(string(data))
		return
	}

	if len(infos) == 0 {
		fmt.Printf("No wallets found in %s\n", dir)
		fmt.Println()
		fmt.Println("Generate a new wallet with:")
		fmt.Println("  monoctl wallet generate --name my-wallet")
		return
	}

	fmt.Printf("Wallets in %s\n", dir)
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-44s %-48s\n", "EVM ADDRESS", "BECH32 ADDRESS")
	fmt.Println(strings.Repeat("-", 80))

	for _, info := range infos {
		fmt.Printf("%-44s %-48s\n", info.EVMAddress, info.Bech32Addr)
		fmt.Printf("  File: %s (created %s)\n", info.Filename, info.CreatedAt.Format("2006-01-02 15:04"))
	}
}

func runWalletInfo(cmd *cobra.Command, args []string) {
	filePath, _ := cmd.Flags().GetString("file")

	ks, err := walletgen.LoadKeystore(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading keystore: %v\n", err)
		os.Exit(1)
	}

	evmAddr := walletgen.GetKeystoreAddress(ks)
	bech32Addr, _ := walletgen.GetKeystoreBech32Address(ks)

	// Get file info
	stat, _ := os.Stat(filePath)

	if jsonOutput {
		out := map[string]interface{}{
			"file":           filePath,
			"evm_address":    evmAddr,
			"bech32_address": bech32Addr,
			"version":        ks.Version,
			"id":             ks.ID,
			"cipher":         ks.Crypto.Cipher,
			"kdf":            ks.Crypto.KDF,
		}
		if stat != nil {
			out["created_at"] = stat.ModTime()
			out["size_bytes"] = stat.Size()
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("Wallet Information")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("File:           %s\n", filePath)
	fmt.Printf("EVM Address:    %s\n", evmAddr)
	fmt.Printf("Bech32 Address: %s\n", bech32Addr)
	fmt.Printf("Keystore ID:    %s\n", ks.ID)
	fmt.Printf("Version:        %d\n", ks.Version)
	fmt.Printf("Cipher:         %s\n", ks.Crypto.Cipher)
	fmt.Printf("KDF:            %s\n", ks.Crypto.KDF)
	if stat != nil {
		fmt.Printf("Created:        %s\n", stat.ModTime().Format("2006-01-02 15:04:05"))
	}
}

// promptPassword prompts for a password without echoing
func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Check if stdin is a terminal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		// Read password without echo
		password, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // Add newline after password input
		if err != nil {
			return "", err
		}
		return string(password), nil
	}

	// Fallback for non-terminal (e.g., piped input)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}

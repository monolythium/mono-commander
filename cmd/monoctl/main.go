// Package main provides the CLI entry point for mono-commander.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/monolythium/mono-commander/internal/core"
	"github.com/monolythium/mono-commander/internal/logs"
	"github.com/monolythium/mono-commander/internal/net"
	oshelpers "github.com/monolythium/mono-commander/internal/os"
	"github.com/monolythium/mono-commander/internal/tui"
	"github.com/spf13/cobra"
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

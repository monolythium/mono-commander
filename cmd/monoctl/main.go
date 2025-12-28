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

// Package main provides the CLI entry point for mono-commander.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	gonet "net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/monolythium/mono-commander/internal/core"
	"github.com/monolythium/mono-commander/internal/logs"
	"github.com/monolythium/mono-commander/internal/mesh"
	"github.com/monolythium/mono-commander/internal/monod"
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
			// Refuse to run as root
			if os.Geteuid() == 0 {
				fmt.Fprintf(os.Stderr, "Error: monoctl must not be run as root\n")
				fmt.Fprintf(os.Stderr, "Run as the user that will own the node home directory.\n")
				os.Exit(1)
			}
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

	peersValidateCmd = &cobra.Command{
		Use:   "validate <peers_string_or_file>",
		Short: "Validate peer address format and reachability",
		Long: `Validate peer addresses for correct format and optional connectivity.

Peer format: nodeID@host:port
  - nodeID: 40-character lowercase hex string
  - host: IP address or hostname
  - port: valid port number (1-65535)

Examples:
  # Validate a single peer
  monoctl peers validate "339b1bbca725378e640f932ee0b8cd51cc638c73@95.217.191.120:26656"

  # Validate multiple peers (comma-separated)
  monoctl peers validate "node1@host1:26656,node2@host2:26656"

  # Validate from file (one peer per line)
  monoctl peers validate /path/to/peers.txt

  # Check connectivity (DNS + TCP)
  monoctl peers validate --check-connectivity "node@host:26656"`,
		Args: cobra.ExactArgs(1),
		Run:  runPeersValidate,
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

	// Version command
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("monoctl version %s\n", tui.Version)
			fmt.Printf("  OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
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

	// Doctor command - explains deployment mode
	doctorCmd = &cobra.Command{
		Use:   "doctor",
		Short: "Explain deployment mode and check configuration",
		Long: `Show what mode Commander operates in and what artifacts it creates.

This command helps clarify that Commander uses host-native execution (no Docker)
and generates systemd units for process management.

Use this command to understand:
  - Deployment mode (host-native vs containerized)
  - File locations (binaries, configs, data)
  - What each command creates`,
		Run: runDoctor,
	}

	// Monod command group
	monodCmd = &cobra.Command{
		Use:   "monod",
		Short: "Monod binary management commands",
		Long: `Install and manage the monod binary from GitHub releases.

Install monod:
  monoctl monod install [--version v0.1.0]

Check installation:
  monoctl monod status`,
	}

	monodInstallCmd = &cobra.Command{
		Use:   "install",
		Short: "Install the monod binary",
		Long: `Download and install the monod binary from GitHub releases.

Automatically detects your OS and architecture, fetches the appropriate binary,
verifies the SHA256 checksum, and installs it to ~/.local/bin/monod (or /usr/local/bin with --system).

Examples:
  monoctl monod install
  monoctl monod install --version v0.1.0
  monoctl monod install --system
  monoctl monod install --dry-run`,
		Run: runMonodInstall,
	}

	monodStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check monod installation status",
		Run:   runMonodStatus,
	}

	// Docker command group
	dockerCmd = &cobra.Command{
		Use:   "docker",
		Short: "Docker-based node deployment",
		Long: `Deploy and manage Monolythium nodes using Docker containers.

This is an alternative to host-native (systemd) deployment, ideal for:
  - Development and testing environments
  - Multi-network node operators
  - Users who prefer container-based deployments
  - Systems without systemd support (including macOS)

Initialize a Docker deployment:
  monoctl docker init --network Sprintnet

Manage the container:
  monoctl docker up
  monoctl docker down
  monoctl docker restart
  monoctl docker logs [--follow]
  monoctl docker status

Upgrade to a new version:
  monoctl docker upgrade --version v0.2.0`,
	}

	dockerInitCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize Docker environment for a network",
		Long: `Download genesis, configure peers, and generate docker-compose.yml.

This command performs:
  1. Downloads genesis.json from the network registry
  2. Validates genesis and verifies SHA256
  3. Downloads peers from registry
  4. Generates docker-compose.yml with proper configuration

Example:
  monoctl docker init --network Sprintnet
  monoctl docker init --network Sprintnet --home ~/.monod --dry-run`,
		Run: runDockerInit,
	}

	dockerUpCmd = &cobra.Command{
		Use:   "up",
		Short: "Start the Docker container",
		Long: `Start the Monolythium node container using docker-compose.

Equivalent to: docker compose up -d`,
		Run: runDockerUp,
	}

	dockerDownCmd = &cobra.Command{
		Use:   "down",
		Short: "Stop the Docker container",
		Long: `Stop and remove the Monolythium node container.

Equivalent to: docker compose down`,
		Run: runDockerDown,
	}

	dockerRestartCmd = &cobra.Command{
		Use:   "restart",
		Short: "Restart the Docker container",
		Long: `Restart the Monolythium node container.

Equivalent to: docker compose restart`,
		Run: runDockerRestart,
	}

	dockerLogsCmd = &cobra.Command{
		Use:   "logs",
		Short: "View container logs",
		Long: `View logs from the Monolythium node container.

Equivalent to: docker compose logs [-f]`,
		Run: runDockerLogs,
	}

	dockerStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show container status",
		Long: `Show the status of the Monolythium node container.

Displays container state, uptime, and basic health information.`,
		Run: runDockerStatus,
	}

	dockerUpgradeCmd = &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade to a new version",
		Long: `Upgrade the Monolythium node to a new Docker image version.

This command:
  1. Pulls the new image version
  2. Updates docker-compose.yml
  3. Restarts the container with the new image

Example:
  monoctl docker upgrade --version v0.2.0`,
		Run: runDockerUpgrade,
	}

	// Node command group - role management
	nodeCmd = &cobra.Command{
		Use:   "node",
		Short: "Node configuration and role management",
		Long: `Manage node configuration and deployment roles.

Node roles determine pruning and seed_mode settings:
  - full_node:    Normal node with pruning enabled, seed_mode=false
  - archive_node: Full history node with pruning=nothing, seed_mode=false
  - seed_node:    Official seed requiring full archive + seed_mode=true

IMPORTANT: seed_mode=true REQUIRES pruning=nothing. Seeds must be full
archive nodes to serve genesis blocksync to new nodes.

Commands:
  monoctl node configure --role <role> --home ~/.monod
  monoctl node role --home ~/.monod`,
	}

	nodeConfigureCmd = &cobra.Command{
		Use:   "configure",
		Short: "Configure node for a specific role",
		Long: `Configure node settings based on deployment role.

Roles:
  full_node    - Normal pruning, seed_mode=false (default)
  archive_node - pruning=nothing, seed_mode=false
  seed_node    - pruning=nothing, seed_mode=true (REQUIRES archive)

CRITICAL: Selecting seed_node will fail if pruning is not set to 'nothing'.
Seeds must be full archive nodes to serve historical blocks.

Examples:
  monoctl node configure --role full_node --home ~/.monod
  monoctl node configure --role archive_node --home ~/.monod
  monoctl node configure --role seed_node --home ~/.monod --dry-run`,
		Run: runNodeConfigure,
	}

	nodeRoleCmd = &cobra.Command{
		Use:   "role",
		Short: "Detect current node role from configuration",
		Long: `Analyze config.toml and app.toml to determine current node role.

Also validates that the configuration is consistent and safe.
Detects dangerous configurations like seed_mode=true with pruning enabled.

Example:
  monoctl node role --home ~/.monod`,
		Run: runNodeRole,
	}

	// Node reset command - safe nuke
	nodeResetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Reset node to clean state (safe nuke)",
		Long: `Completely reset the node home directory to a clean state.

This command will:
  1. Stop the monod systemd service (if running)
  2. Delete the data directory (blockchain data)
  3. Delete config files (config.toml, app.toml, genesis.json)
  4. Optionally preserve or delete node_key.json and priv_validator_key.json
  5. Recreate required directories with correct ownership

WARNINGS:
  - This is destructive and cannot be undone
  - Blockchain data will be lost
  - You will need to re-sync from genesis or snapshot

Use --preserve-keys to keep node_key.json and priv_validator_key.json.
Use --force to skip confirmation prompt.

Examples:
  monoctl node reset --home ~/.monod
  monoctl node reset --home ~/.monod --preserve-keys
  monoctl node reset --home ~/.monod --force`,
		Run: runNodeReset,
	}

	// Config command group - configuration drift detection and repair
	configCmd = &cobra.Command{
		Use:   "config",
		Short: "Configuration drift detection and repair",
		Long: `Detect and repair configuration drift from canonical network values.

The canonical source of truth for network configuration is the monolythium/networks
repository. This command group helps ensure your node configuration matches the
canonical values to prevent consensus failures.

Commands:
  monoctl config doctor --network <network>  # Detect configuration drift
  monoctl config repair --network <network>  # Repair configuration drift`,
	}

	configDoctorCmd = &cobra.Command{
		Use:   "doctor",
		Short: "Detect configuration drift from canonical values",
		Long: `Check node configuration against canonical network values.

Detects drift in critical configuration fields:
  - chain-id in client.toml (CRITICAL - causes consensus failure)
  - evm-chain-id in app.toml (CRITICAL - causes AppHash mismatch)
  - seeds in config.toml (WARNING - affects connectivity)
  - persistent_peers in config.toml (WARNING - affects connectivity)

CRITICAL drift must be fixed immediately as it will cause consensus failures.
WARNING drift affects network connectivity but won't cause consensus issues.

Examples:
  monoctl config doctor --network Sprintnet --home ~/.monod
  monoctl config doctor --network Sprintnet --home ~/.monod --json`,
		Run: runConfigDoctor,
	}

	configRepairCmd = &cobra.Command{
		Use:   "repair",
		Short: "Repair configuration drift to canonical values",
		Long: `Repair configuration drift by updating TOML files to canonical values.

This command regenerates configuration values from the canonical network config:
  - Updates chain-id in client.toml
  - Updates evm-chain-id in app.toml
  - Updates seeds in config.toml
  - Updates persistent_peers in config.toml

Keys are preserved:
  - node_key.json (node identity)
  - priv_validator_key.json (validator signing key)

Use --dry-run to preview changes without applying them.

Examples:
  monoctl config repair --network Sprintnet --home ~/.monod --dry-run
  monoctl config repair --network Sprintnet --home ~/.monod`,
		Run: runConfigRepair,
	}
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	// Version command
	rootCmd.AddCommand(versionCmd)

	// Networks subcommands
	networksCmd.AddCommand(networksListCmd)
	rootCmd.AddCommand(networksCmd)

	// Join command flags
	joinCmd.Flags().String("network", "", "Network to join (Localnet, Sprintnet, Testnet, Mainnet)")
	joinCmd.Flags().String("genesis-url", "", "Genesis file URL (uses network default if not specified)")
	joinCmd.Flags().String("genesis-sha256", "", "Expected SHA256 of genesis file")
	joinCmd.Flags().String("peers-url", "", "Peers registry URL (uses network default if not specified)")
	joinCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	joinCmd.Flags().String("moniker", "", "Node moniker (auto-generated from hostname if not specified)")
	joinCmd.Flags().String("monod-path", "", "Path to monod binary (auto-detected if not specified)")
	joinCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	joinCmd.Flags().Bool("bootstrap", false, "Use bootstrap mode: trusted peers only, pex=false (recommended for deterministic sync)")
	joinCmd.Flags().Bool("clear-addrbook", false, "Clear addrbook.json to avoid poisoned peers")
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

	// Peers validate command flags
	peersValidateCmd.Flags().Bool("check-connectivity", false, "Check DNS resolution and TCP connectivity")
	peersCmd.AddCommand(peersValidateCmd)

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

	// Monod commands
	monodInstallCmd.Flags().String("version", "", "Version to install (default: latest)")
	monodInstallCmd.Flags().String("url", "", "Download URL for the binary (auto-detected if not specified)")
	monodInstallCmd.Flags().String("sha256", "", "Expected SHA256 checksum (auto-fetched if not specified)")
	monodInstallCmd.Flags().Bool("insecure", false, "Skip checksum verification (not recommended)")
	monodInstallCmd.Flags().Bool("system", false, "Install to /usr/local/bin (requires sudo)")
	monodInstallCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	monodCmd.AddCommand(monodInstallCmd)

	monodStatusCmd.Flags().Bool("system", false, "Check /usr/local/bin instead of ~/.local/bin")
	monodCmd.AddCommand(monodStatusCmd)

	rootCmd.AddCommand(monodCmd)

	// Doctor command (no flags needed)
	rootCmd.AddCommand(doctorCmd)

	// Docker commands
	dockerInitCmd.Flags().String("network", "", "Network to join (Sprintnet, Testnet, Mainnet)")
	dockerInitCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	dockerInitCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	dockerInitCmd.MarkFlagRequired("network")
	dockerCmd.AddCommand(dockerInitCmd)

	dockerUpCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	dockerCmd.AddCommand(dockerUpCmd)

	dockerDownCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	dockerCmd.AddCommand(dockerDownCmd)

	dockerRestartCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	dockerCmd.AddCommand(dockerRestartCmd)

	dockerLogsCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	dockerLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	dockerLogsCmd.Flags().IntP("lines", "n", 100, "Number of lines to show")
	dockerCmd.AddCommand(dockerLogsCmd)

	dockerStatusCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	dockerCmd.AddCommand(dockerStatusCmd)

	dockerUpgradeCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	dockerUpgradeCmd.Flags().String("version", "", "Version tag to upgrade to (e.g., v0.2.0)")
	dockerUpgradeCmd.MarkFlagRequired("version")
	dockerCmd.AddCommand(dockerUpgradeCmd)

	rootCmd.AddCommand(dockerCmd)

	// Node commands
	nodeConfigureCmd.Flags().String("role", "", "Node role (full_node, archive_node, seed_node)")
	nodeConfigureCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	nodeConfigureCmd.Flags().String("network", "", "Network name (for context)")
	nodeConfigureCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	nodeConfigureCmd.MarkFlagRequired("role")
	nodeCmd.AddCommand(nodeConfigureCmd)

	nodeRoleCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	nodeCmd.AddCommand(nodeRoleCmd)

	// Node reset flags
	nodeResetCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	nodeResetCmd.Flags().Bool("preserve-keys", false, "Preserve node_key.json and priv_validator_key.json")
	nodeResetCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	nodeResetCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	nodeCmd.AddCommand(nodeResetCmd)

	rootCmd.AddCommand(nodeCmd)

	// Config commands - drift detection and repair
	configDoctorCmd.Flags().String("network", "", "Network name (Sprintnet, Testnet, Mainnet)")
	configDoctorCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	configDoctorCmd.MarkFlagRequired("network")
	configCmd.AddCommand(configDoctorCmd)

	configRepairCmd.Flags().String("network", "", "Network name (Sprintnet, Testnet, Mainnet)")
	configRepairCmd.Flags().String("home", "", "Node home directory (default: ~/.monod)")
	configRepairCmd.Flags().Bool("dry-run", false, "Preview changes without applying them")
	configRepairCmd.MarkFlagRequired("network")
	configCmd.AddCommand(configRepairCmd)

	rootCmd.AddCommand(configCmd)
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
	// Refuse to run as root
	if os.Geteuid() == 0 {
		fmt.Fprintf(os.Stderr, "Error: monoctl must not be run as root\n")
		fmt.Fprintf(os.Stderr, "Run as the user that will own the node home directory.\n")
		os.Exit(1)
	}

	networkStr, _ := cmd.Flags().GetString("network")
	genesisURL, _ := cmd.Flags().GetString("genesis-url")
	genesisSHA, _ := cmd.Flags().GetString("genesis-sha256")
	peersURL, _ := cmd.Flags().GetString("peers-url")
	home, _ := cmd.Flags().GetString("home")
	moniker, _ := cmd.Flags().GetString("moniker")
	monodPath, _ := cmd.Flags().GetString("monod-path")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	bootstrap, _ := cmd.Flags().GetBool("bootstrap")
	clearAddrbook, _ := cmd.Flags().GetBool("clear-addrbook")

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

	// Determine sync strategy
	syncStrategy := core.SyncStrategyDefault
	if bootstrap {
		syncStrategy = core.SyncStrategyBootstrap
	}

	opts := core.JoinOptions{
		Network:       network,
		Home:          home,
		GenesisURL:    genesisURL,
		GenesisSHA:    genesisSHA,
		PeersURL:      peersURL,
		DryRun:        dryRun,
		Logger:        logger,
		SyncStrategy:  syncStrategy,
		ClearAddrbook: clearAddrbook,
		Moniker:       moniker,
		MonodPath:     monodPath,
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
	if bootstrap {
		fmt.Println("Mode: BOOTSTRAP (trusted peers only, pex=false)")
		fmt.Println("      Using bootstrap_peers for deterministic genesis sync")
	}
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
	fmt.Printf("Node home: %s\n", home)
	fmt.Printf("Chain ID: %s\n", result.ChainID)
	if result.NodeID != "" {
		fmt.Printf("Node ID: %s\n", result.NodeID)
	}

	if dryRun {
		fmt.Println("\nConfig that would be applied:")
		fmt.Println(result.ConfigPatch)
	}

	if result.Success && !dryRun {
		fmt.Println()
		fmt.Println("Node is ready. Next steps:")
		fmt.Println("  1. Install systemd service: monoctl systemd install --network", networkStr, "--user", os.Getenv("USER"))
		fmt.Println("  2. Start the node: sudo systemctl start monod")
		fmt.Println("  3. Check status: monoctl status --network", networkStr)
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

	seeds := reg.Seeds
	peers := core.MergePeers(reg.Peers, reg.PersistentPeers)
	patch := core.GenerateConfigPatch(seeds, peers)
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
	// Check if systemd is available (Linux only)
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "Error: systemd is only available on Linux\n")
		fmt.Fprintf(os.Stderr, "On %s, use Docker mode instead:\n", runtime.GOOS)
		fmt.Fprintf(os.Stderr, "  monoctl docker init --network <network>\n")
		fmt.Fprintf(os.Stderr, "  monoctl docker up\n")
		os.Exit(1)
	}

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

	// Ensure home is an absolute path (no ~ or relative paths)
	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, ".monod")
	} else if strings.HasPrefix(home, "~") {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, strings.TrimPrefix(home, "~"))
	}
	// Convert to absolute path
	home, _ = filepath.Abs(home)

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

// =============================================================================
// Monod Commands
// =============================================================================

func runMonodInstall(cmd *cobra.Command, args []string) {
	url, _ := cmd.Flags().GetString("url")
	sha256sum, _ := cmd.Flags().GetString("sha256")
	version, _ := cmd.Flags().GetString("version")
	insecure, _ := cmd.Flags().GetBool("insecure")
	useSystem, _ := cmd.Flags().GetBool("system")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	opts := monod.InstallOptions{
		URL:           url,
		SHA256:        sha256sum,
		Version:       version,
		UseSystemPath: useSystem,
		Insecure:      insecure,
		DryRun:        dryRun,
	}

	result := monod.Install(opts)

	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		if !result.Success {
			os.Exit(1)
		}
		return
	}

	fmt.Println("Monod Binary Installation")
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
		fmt.Printf("Version: %s\n", result.Version)

		if !opts.UseSystemPath {
			fmt.Println("\nMake sure ~/.local/bin is in your PATH:")
			fmt.Println("  export PATH=\"$HOME/.local/bin:$PATH\"")
		}
	}
}

func runMonodStatus(cmd *cobra.Command, args []string) {
	useSystem, _ := cmd.Flags().GetBool("system")

	installPath := monod.BinaryInstallPath(useSystem)
	exists := monod.BinaryExists(useSystem)

	if jsonOutput {
		out := map[string]interface{}{
			"install_path": installPath,
			"installed":    exists,
		}
		if exists {
			version, _ := monod.GetInstalledVersion(useSystem)
			out["version"] = version
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("Monod Binary Status")
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Install path:  %s\n", installPath)

	if exists {
		fmt.Printf("Status:        INSTALLED\n")
		version, err := monod.GetInstalledVersion(useSystem)
		if err == nil {
			fmt.Printf("Version:       %s\n", version)
		}
	} else {
		fmt.Printf("Status:        NOT INSTALLED\n")
		fmt.Println()
		fmt.Println("To install, run:")
		fmt.Println("  monoctl monod install")
	}
}

// =============================================================================
// Doctor Command
// =============================================================================

func runDoctor(cmd *cobra.Command, args []string) {
	homeDir, _ := os.UserHomeDir()

	if jsonOutput {
		// Detect deployment mode
		deployMode := "host-native"
		nodeHome := filepath.Join(homeDir, ".monod")
		if _, err := os.Stat(filepath.Join(nodeHome, "docker-compose.yml")); err == nil {
			deployMode = "docker"
		}

		// Load commander config for network info
		cfgPath := filepath.Join(homeDir, ".mono-commander", "config.json")
		var network string
		var rpcEndpoint string
		if data, err := os.ReadFile(cfgPath); err == nil {
			var cfg map[string]interface{}
			if json.Unmarshal(data, &cfg) == nil {
				if n, ok := cfg["selected_network"].(string); ok {
					network = n
				}
			}
		}
		if network == "" {
			network = "unknown"
		}

		// Determine RPC endpoint
		rpcEndpoint = "http://localhost:26657"
		if deployMode == "docker" {
			rpcEndpoint = "http://localhost:26657"
		}

		// Check RPC status
		rpcReachable := false
		var height int64
		var catchingUp bool
		var peerCount int

		conn, _ := gonet.DialTimeout("tcp", "localhost:26657", 3*time.Second)
		if conn != nil {
			conn.Close()
			rpcReachable = true
			// Try to get status from RPC
			opts := core.StatusOptions{
				Endpoints: core.Endpoints{
					CometRPC: rpcEndpoint,
				},
			}
			if status, err := core.GetNodeStatus(opts); err == nil {
				height = status.LatestHeight
				catchingUp = status.CatchingUp
				peerCount = status.PeersCount
			}
		}

		// Check installation status
		monodInstalled := false
		monodUserPath := filepath.Join(homeDir, ".local", "bin", "monod")
		monodSysPath := "/usr/local/bin/monod"
		if _, err := os.Stat(monodUserPath); err == nil {
			monodInstalled = true
		} else if _, err := os.Stat(monodSysPath); err == nil {
			monodInstalled = true
		}

		nodeHomeExists := false
		if _, err := os.Stat(nodeHome); err == nil {
			nodeHomeExists = true
		}

		out := map[string]interface{}{
			"deployment_mode": deployMode,
			"network":         network,
			"rpc_endpoint":    rpcEndpoint,
			"rpc_reachable":   rpcReachable,
			"height":          height,
			"catching_up":     catchingUp,
			"peer_count":      peerCount,
			"status": map[string]bool{
				"monod_installed":  monodInstalled,
				"node_home_exists": nodeHomeExists,
			},
			"locations": map[string]string{
				"monod_user":       monodUserPath,
				"monod_system":     monodSysPath,
				"node_home":        nodeHome,
				"commander_config": filepath.Join(homeDir, ".mono-commander"),
				"systemd_units":    "/etc/systemd/system/",
			},
		}

		// Add role validation if node home exists
		configPath := filepath.Join(nodeHome, "config", "config.toml")
		if _, err := os.Stat(configPath); err == nil {
			role, err := core.DetectCurrentRole(nodeHome)
			if err == nil {
				roleInfo := map[string]interface{}{
					"detected_role": role,
					"description":   core.RoleDescription(role),
				}

				validation, err := core.ValidateRoleConfig(nodeHome, role)
				if err == nil {
					issues := make([]map[string]string, len(validation.Issues))
					for i, issue := range validation.Issues {
						issues[i] = map[string]string{
							"severity": issue.Severity,
							"field":    issue.Field,
							"expected": issue.Expected,
							"actual":   issue.Actual,
							"message":  issue.Message,
						}
					}
					roleInfo["valid"] = validation.Valid
					roleInfo["issues"] = issues
					roleInfo["suggestions"] = validation.Suggestions
				}
				out["role_validation"] = roleInfo
			}
		}

		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("Mono Commander - Deployment Mode")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	fmt.Println("DEPLOYMENT MODE")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("  Mode:            HOST-NATIVE")
	fmt.Println("  Container:       NO (no Docker/docker-compose)")
	fmt.Println("  Process Manager: systemd (Linux)")
	fmt.Println()

	fmt.Println("FILE LOCATIONS")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  monod binary (user):   %s\n", filepath.Join(homeDir, ".local", "bin", "monod"))
	fmt.Printf("  monod binary (system): /usr/local/bin/monod\n")
	fmt.Printf("  Node home:             %s\n", filepath.Join(homeDir, ".monod"))
	fmt.Printf("  Commander config:      %s\n", filepath.Join(homeDir, ".mono-commander"))
	fmt.Printf("  Systemd units:         /etc/systemd/system/\n")
	fmt.Println()

	fmt.Println("COMMAND → ARTIFACT MAPPING")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("  monoctl monod install   → ~/.local/bin/monod")
	fmt.Println("  monoctl join            → ~/.monod/config/genesis.json")
	fmt.Println("                          → ~/.monod/config/config-patch.toml")
	fmt.Println("  monoctl systemd install → /etc/systemd/system/monod-<network>.service")
	fmt.Println("  monoctl mesh enable     → ~/.mono-commander/mesh-<network>.toml")
	fmt.Println("                          → /etc/systemd/system/mesh-<network>.service")
	fmt.Println("  monoctl wallet generate → ~/.mono-commander/wallets/<name>.json")
	fmt.Println()

	// Check current installation status
	fmt.Println("CURRENT STATUS")
	fmt.Println(strings.Repeat("-", 40))

	// Check monod
	monodUserPath := filepath.Join(homeDir, ".local", "bin", "monod")
	monodSysPath := "/usr/local/bin/monod"

	if _, err := os.Stat(monodUserPath); err == nil {
		fmt.Printf("  monod (user):   INSTALLED at %s\n", monodUserPath)
	} else if _, err := os.Stat(monodSysPath); err == nil {
		fmt.Printf("  monod (system): INSTALLED at %s\n", monodSysPath)
	} else {
		fmt.Println("  monod:          NOT INSTALLED")
	}

	// Check node home
	nodeHome := filepath.Join(homeDir, ".monod")
	if _, err := os.Stat(nodeHome); err == nil {
		fmt.Printf("  Node home:      EXISTS at %s\n", nodeHome)
	} else {
		fmt.Println("  Node home:      NOT INITIALIZED")
	}

	// Check commander config
	cmdConfig := filepath.Join(homeDir, ".mono-commander")
	if _, err := os.Stat(cmdConfig); err == nil {
		fmt.Printf("  Commander:      CONFIGURED at %s\n", cmdConfig)
	} else {
		fmt.Println("  Commander:      NOT CONFIGURED (will be created on first use)")
	}

	// Role validation (only if node home exists)
	configPath := filepath.Join(nodeHome, "config", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Println()
		fmt.Println("NODE ROLE VALIDATION")
		fmt.Println(strings.Repeat("-", 40))

		role, err := core.DetectCurrentRole(nodeHome)
		if err != nil {
			fmt.Printf("  Error detecting role: %v\n", err)
		} else {
			fmt.Printf("  Detected Role: %s\n", role)
			fmt.Printf("  Description:   %s\n", core.RoleDescription(role))

			validation, err := core.ValidateRoleConfig(nodeHome, role)
			if err != nil {
				fmt.Printf("  Validation error: %v\n", err)
			} else {
				hasCritical := false
				hasWarning := false
				for _, issue := range validation.Issues {
					if issue.Severity == "CRITICAL" {
						hasCritical = true
					}
					if issue.Severity == "WARNING" {
						hasWarning = true
					}
				}

				if validation.Valid {
					fmt.Println("  Status:        VALID")
				} else if hasCritical {
					fmt.Println("  Status:        CRITICAL ISSUES DETECTED")
				} else if hasWarning {
					fmt.Println("  Status:        WARNINGS")
				}

				for _, issue := range validation.Issues {
					var marker string
					switch issue.Severity {
					case "CRITICAL":
						marker = "CRITICAL"
					case "WARNING":
						marker = "WARNING"
					default:
						marker = "INFO"
					}
					fmt.Printf("\n  [%s] %s\n", marker, issue.Message)
					if issue.Severity == "CRITICAL" {
						fmt.Printf("    Expected: %s\n", issue.Expected)
						fmt.Printf("    Actual:   %s\n", issue.Actual)
					}
				}

				// Special warning for seed nodes
				if role == core.RoleSeedNode {
					fmt.Println()
					fmt.Println("  SEED NODE REQUIREMENTS:")
					fmt.Println("    - Must have pruning=nothing")
					fmt.Println("    - Must be synced from genesis (earliest_block_height=1)")
					fmt.Println("    - Must be able to serve historical blocks")
					fmt.Println()
					fmt.Println("  Run: monoctl node role --home ~/.monod")
					fmt.Println("  to verify earliest_block_height via RPC")
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("TIP: Use --dry-run on any write command to preview changes.")
	fmt.Println("     Use 'monoctl node role' to check node role and configuration.")
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

// Docker command handlers

func runDockerInit(cmd *cobra.Command, args []string) {
	networkName, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Resolve home directory
	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, ".monod")
	} else if strings.HasPrefix(home, "~") {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, strings.TrimPrefix(home, "~"))
	}
	home, _ = filepath.Abs(home)

	network, err := core.GetNetwork(core.NetworkName(networkName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid network %s: %v\n", networkName, err)
		os.Exit(1)
	}

	// Create logger
	logger := slog.Default()

	// Step 1: Run join to download genesis and configure peers
	fmt.Println("Docker Init: " + networkName)
	if dryRun {
		fmt.Println("(DRY RUN - no changes will be made)")
	}
	fmt.Println()

	joinOpts := core.JoinOptions{
		Network: network.Name,
		Home:    home,
		DryRun:  dryRun,
		Logger:  logger,
	}

	fetcher := net.NewHTTPFetcher()
	result, err := core.Join(joinOpts, fetcher)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during join: %v\n", err)
		os.Exit(1)
	}

	// Print join steps
	for _, step := range result.Steps {
		status := "[+]"
		if step.Status == "failed" {
			status = "[X]"
		} else if step.Status == "skipped" {
			status = "[-]"
		}
		msg := step.Name
		if step.Message != "" {
			msg += ": " + step.Message
		}
		fmt.Printf("%s %s\n", status, msg)
	}

	// Step 2: Generate docker-compose.yml
	fmt.Println()
	fmt.Println("[+] Generating docker-compose.yml")

	composeContent := generateDockerCompose(network, home, result.ConfigPatch)
	composePath := filepath.Join(home, "docker-compose.yml")

	if dryRun {
		fmt.Printf("Would write to: %s\n", composePath)
		fmt.Println("\nContent preview:")
		fmt.Println(composeContent)
	} else {
		// Ensure directory exists
		if err := os.MkdirAll(home, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing docker-compose.yml: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Written to: %s\n", composePath)
	}

	fmt.Println()
	fmt.Println("Docker environment initialized!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", home)
	fmt.Println("  monoctl docker up")
}

func runDockerUp(cmd *cobra.Command, args []string) {
	home := getDockerHome(cmd)
	composePath := filepath.Join(home, "docker-compose.yml")

	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: docker-compose.yml not found at %s\n", composePath)
		fmt.Fprintf(os.Stderr, "Run 'monoctl docker init --network <network>' first\n")
		os.Exit(1)
	}

	fmt.Println("Starting container...")
	if err := runDockerCompose(home, "up", "-d"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Container started successfully")
}

func runDockerDown(cmd *cobra.Command, args []string) {
	home := getDockerHome(cmd)

	fmt.Println("Stopping container...")
	if err := runDockerCompose(home, "down"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Container stopped")
}

func runDockerRestart(cmd *cobra.Command, args []string) {
	home := getDockerHome(cmd)

	fmt.Println("Restarting container...")
	if err := runDockerCompose(home, "restart"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Container restarted")
}

func runDockerLogs(cmd *cobra.Command, args []string) {
	home := getDockerHome(cmd)
	follow, _ := cmd.Flags().GetBool("follow")
	lines, _ := cmd.Flags().GetInt("lines")

	dockerArgs := []string{"logs", fmt.Sprintf("--tail=%d", lines)}
	if follow {
		dockerArgs = append(dockerArgs, "-f")
	}

	if err := runDockerCompose(home, dockerArgs...); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDockerStatus(cmd *cobra.Command, args []string) {
	home := getDockerHome(cmd)
	composePath := filepath.Join(home, "docker-compose.yml")

	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		fmt.Println("Status: NOT INITIALIZED")
		fmt.Printf("No docker-compose.yml found at %s\n", home)
		return
	}

	fmt.Printf("Docker Compose: %s\n", composePath)
	fmt.Println()

	if err := runDockerCompose(home, "ps"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDockerUpgrade(cmd *cobra.Command, args []string) {
	home := getDockerHome(cmd)
	version, _ := cmd.Flags().GetString("version")

	composePath := filepath.Join(home, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: docker-compose.yml not found at %s\n", composePath)
		os.Exit(1)
	}

	// Read existing compose file to get current config
	content, err := os.ReadFile(composePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading docker-compose.yml: %v\n", err)
		os.Exit(1)
	}

	// Update image version in compose file
	oldImage := "monolythium/monod:latest"
	newImage := fmt.Sprintf("monolythium/monod:%s", version)
	newContent := strings.ReplaceAll(string(content), oldImage, newImage)

	// Also handle if there's already a versioned image
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "image: monolythium/monod:") {
			lines[i] = fmt.Sprintf("    image: %s", newImage)
		}
	}
	newContent = strings.Join(lines, "\n")

	fmt.Printf("Upgrading to version: %s\n", version)

	// Write updated compose file
	if err := os.WriteFile(composePath, []byte(newContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing docker-compose.yml: %v\n", err)
		os.Exit(1)
	}

	// Pull new image
	fmt.Println("Pulling new image...")
	if err := runDockerCompose(home, "pull"); err != nil {
		fmt.Fprintf(os.Stderr, "Error pulling image: %v\n", err)
		os.Exit(1)
	}

	// Restart with new image
	fmt.Println("Restarting container...")
	if err := runDockerCompose(home, "up", "-d"); err != nil {
		fmt.Fprintf(os.Stderr, "Error restarting: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Upgraded to %s successfully\n", version)
}

// Helper functions for Docker commands

func getDockerHome(cmd *cobra.Command) string {
	home, _ := cmd.Flags().GetString("home")
	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, ".monod")
	} else if strings.HasPrefix(home, "~") {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, strings.TrimPrefix(home, "~"))
	}
	home, _ = filepath.Abs(home)
	return home
}

func runDockerCompose(workDir string, args ...string) error {
	// Try docker compose (v2) first, fall back to docker-compose (v1)
	fullArgs := append([]string{"compose"}, args...)

	cmd := exec.Command("docker", fullArgs...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		// Try docker-compose as fallback
		cmd = exec.Command("docker-compose", args...)
		cmd.Dir = workDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}

func generateDockerCompose(network core.Network, home, configPatch string) string {
	// Extract seeds and persistent_peers from config patch if available
	var seeds, persistentPeers string
	for _, line := range strings.Split(configPatch, "\n") {
		if strings.HasPrefix(line, "seeds = ") {
			seeds = strings.Trim(strings.TrimPrefix(line, "seeds = "), "\"")
		}
		if strings.HasPrefix(line, "persistent_peers = ") {
			persistentPeers = strings.Trim(strings.TrimPrefix(line, "persistent_peers = "), "\"")
		}
	}

	// Build environment variables
	var envVars strings.Builder
	if seeds != "" {
		envVars.WriteString(fmt.Sprintf("      - MONOD_P2P_SEEDS=%s\n", seeds))
	}
	if persistentPeers != "" {
		envVars.WriteString(fmt.Sprintf("      - MONOD_P2P_PERSISTENT_PEERS=%s\n", persistentPeers))
	}

	template := `# Monolythium Node Docker Compose
# Network: %s
# Generated by monoctl

services:
  monod:
    image: monolythium/monod:latest
    container_name: monod-%s
    restart: unless-stopped
    volumes:
      - %s:/root/.monod
    ports:
      - "26656:26656"  # P2P
      - "26657:26657"  # RPC
      - "1317:1317"    # REST API
      - "9090:9090"    # gRPC
      - "8545:8545"    # EVM JSON-RPC
      - "8546:8546"    # EVM WebSocket
    environment:
      - MONOD_CHAIN_ID=%s
%s    command: start --home /root/.monod
`

	return fmt.Sprintf(template,
		network.Name,
		strings.ToLower(string(network.Name)),
		home,
		network.ChainID,
		envVars.String(),
	)
}

// PeerValidationResult holds the result of validating a single peer
type PeerValidationResult struct {
	Peer          string `json:"peer"`
	Valid         bool   `json:"valid"`
	NodeID        string `json:"node_id,omitempty"`
	Host          string `json:"host,omitempty"`
	Port          int    `json:"port,omitempty"`
	Error         string `json:"error,omitempty"`
	DNSResolved   *bool  `json:"dns_resolved,omitempty"`
	TCPReachable  *bool  `json:"tcp_reachable,omitempty"`
	ReachableNote string `json:"reachable_note,omitempty"`
}

// peerRegex validates nodeID@host:port format
var peerRegex = regexp.MustCompile(`^([a-f0-9]{40})@([a-zA-Z0-9.\-]+):(\d+)$`)

func runPeersValidate(cmd *cobra.Command, args []string) {
	input := args[0]
	checkConn, _ := cmd.Flags().GetBool("check-connectivity")

	var peers []string

	// Check if input is a file
	if _, err := os.Stat(input); err == nil {
		data, err := os.ReadFile(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				peers = append(peers, line)
			}
		}
	} else {
		// Treat as comma-separated string
		for _, p := range strings.Split(input, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				peers = append(peers, p)
			}
		}
	}

	if len(peers) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no peers to validate\n")
		os.Exit(1)
	}

	var results []PeerValidationResult
	allValid := true

	for _, peer := range peers {
		result := validatePeer(peer, checkConn)
		results = append(results, result)
		if !result.Valid {
			allValid = false
		}
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Validating %d peer(s)...\n\n", len(peers))
		for _, r := range results {
			if r.Valid {
				fmt.Printf("✓ %s\n", r.Peer)
				fmt.Printf("  NodeID: %s\n", r.NodeID)
				fmt.Printf("  Host:   %s\n", r.Host)
				fmt.Printf("  Port:   %d\n", r.Port)
				if r.DNSResolved != nil {
					if *r.DNSResolved {
						fmt.Printf("  DNS:    resolved\n")
					} else {
						fmt.Printf("  DNS:    FAILED to resolve\n")
					}
				}
				if r.TCPReachable != nil {
					if *r.TCPReachable {
						fmt.Printf("  TCP:    reachable\n")
					} else {
						fmt.Printf("  TCP:    NOT reachable (%s)\n", r.ReachableNote)
					}
				}
			} else {
				fmt.Printf("✗ %s\n", r.Peer)
				fmt.Printf("  Error: %s\n", r.Error)
			}
			fmt.Println()
		}

		if allValid {
			fmt.Println("All peers are valid.")
		} else {
			fmt.Println("Some peers have errors.")
			os.Exit(1)
		}
	}
}

func validatePeer(peer string, checkConn bool) PeerValidationResult {
	result := PeerValidationResult{Peer: peer}

	matches := peerRegex.FindStringSubmatch(peer)
	if matches == nil {
		result.Valid = false
		result.Error = "invalid format: expected nodeID@host:port (nodeID must be 40 hex chars)"
		return result
	}

	result.NodeID = matches[1]
	result.Host = matches[2]
	port, err := strconv.Atoi(matches[3])
	if err != nil || port < 1 || port > 65535 {
		result.Valid = false
		result.Error = fmt.Sprintf("invalid port: %s (must be 1-65535)", matches[3])
		return result
	}
	result.Port = port
	result.Valid = true

	if checkConn {
		// DNS resolution check
		_, err := gonet.LookupHost(result.Host)
		dnsOk := err == nil
		result.DNSResolved = &dnsOk

		// TCP connectivity check (3 second timeout)
		addr := fmt.Sprintf("%s:%d", result.Host, result.Port)
		conn, err := gonet.DialTimeout("tcp", addr, 3*time.Second)
		tcpOk := err == nil
		result.TCPReachable = &tcpOk
		if err != nil {
			result.ReachableNote = err.Error()
		}
		if conn != nil {
			conn.Close()
		}
	}

	return result
}

// =============================================================================
// Node Command Handlers
// =============================================================================

func runNodeConfigure(cmd *cobra.Command, args []string) {
	roleStr, _ := cmd.Flags().GetString("role")
	home, _ := cmd.Flags().GetString("home")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Parse role
	role, err := core.ParseNodeRole(roleStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Default home
	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, ".monod")
	} else if strings.HasPrefix(home, "~") {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, strings.TrimPrefix(home, "~"))
	}
	home, _ = filepath.Abs(home)

	// Check that home directory exists
	configPath := filepath.Join(home, "config", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: config.toml not found at %s\n", configPath)
		fmt.Fprintf(os.Stderr, "Initialize the node first with: monod init <moniker> --home %s\n", home)
		os.Exit(1)
	}

	appPath := filepath.Join(home, "config", "app.toml")
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: app.toml not found at %s\n", appPath)
		fmt.Fprintf(os.Stderr, "Initialize the node first with: monod init <moniker> --home %s\n", home)
		os.Exit(1)
	}

	// For seed_node, check if pruning is already nothing (or we need to set it)
	if role == core.RoleSeedNode {
		allowed, reason := core.IsSeedModeAllowed(home)
		if !allowed && !dryRun {
			// We'll set pruning=nothing as part of configuration
			fmt.Println("Note: seed_node role requires pruning=nothing. Will configure accordingly.")
		}
		_ = allowed
		_ = reason
	}

	expectedConfig := core.GetRoleConfig(role)

	if jsonOutput {
		out := map[string]interface{}{
			"home":    home,
			"role":    role,
			"dry_run": dryRun,
			"config": map[string]interface{}{
				"seed_mode":            expectedConfig.SeedMode,
				"pruning":              expectedConfig.Pruning,
				"pruning_keep_recent":  expectedConfig.PruningKeepRecent,
				"pruning_interval":     expectedConfig.PruningInterval,
			},
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		if dryRun {
			return
		}
	}

	if !jsonOutput {
		fmt.Printf("Node Role Configuration\n")
		fmt.Println(strings.Repeat("=", 50))
		fmt.Printf("Home:     %s\n", home)
		fmt.Printf("Role:     %s\n", role)
		fmt.Printf("Dry Run:  %t\n", dryRun)
		fmt.Println()
		fmt.Println("Configuration to apply:")
		fmt.Printf("  seed_mode:            %t\n", expectedConfig.SeedMode)
		fmt.Printf("  pruning:              %s\n", expectedConfig.Pruning)
		fmt.Printf("  pruning-keep-recent:  %s\n", expectedConfig.PruningKeepRecent)
		fmt.Printf("  pruning-interval:     %s\n", expectedConfig.PruningInterval)
		fmt.Println()

		if dryRun {
			fmt.Println("(DRY RUN - no changes made)")
			return
		}
	}

	// Apply configuration
	if err := core.ApplyRoleConfig(home, role, dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying configuration: %v\n", err)
		os.Exit(1)
	}

	if !jsonOutput {
		fmt.Println("Configuration applied successfully!")
		fmt.Println()
		fmt.Println("IMPORTANT: Restart the node for changes to take effect:")
		fmt.Println("  sudo systemctl restart monod")
		if role == core.RoleSeedNode {
			fmt.Println()
			fmt.Println("WARNING: As a seed node, ensure this node:")
			fmt.Println("  1. Was synced from genesis (not state-synced)")
			fmt.Println("  2. Has earliest_block_height = 1")
			fmt.Println("  3. Can serve historical blocks to new nodes")
		}
	}
}

func runNodeRole(cmd *cobra.Command, args []string) {
	home, _ := cmd.Flags().GetString("home")

	// Default home
	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, ".monod")
	} else if strings.HasPrefix(home, "~") {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, strings.TrimPrefix(home, "~"))
	}
	home, _ = filepath.Abs(home)

	// Check that home directory exists
	configPath := filepath.Join(home, "config", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: config.toml not found at %s\n", configPath)
		fmt.Fprintf(os.Stderr, "Initialize the node first with: monod init <moniker> --home %s\n", home)
		os.Exit(1)
	}

	// Detect current role
	role, err := core.DetectCurrentRole(home)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting role: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration for detected role
	validation, err := core.ValidateRoleConfig(home, role)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error validating configuration: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		type issueJSON struct {
			Severity string `json:"severity"`
			Field    string `json:"field"`
			Expected string `json:"expected"`
			Actual   string `json:"actual"`
			Message  string `json:"message"`
		}

		issues := make([]issueJSON, len(validation.Issues))
		for i, issue := range validation.Issues {
			issues[i] = issueJSON{
				Severity: issue.Severity,
				Field:    issue.Field,
				Expected: issue.Expected,
				Actual:   issue.Actual,
				Message:  issue.Message,
			}
		}

		out := map[string]interface{}{
			"home":        home,
			"role":        role,
			"description": core.RoleDescription(role),
			"valid":       validation.Valid,
			"issues":      issues,
			"suggestions": validation.Suggestions,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Node Role Detection\n")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Home:        %s\n", home)
	fmt.Printf("Role:        %s\n", role)
	fmt.Printf("Description: %s\n", core.RoleDescription(role))
	fmt.Println()

	if len(validation.Issues) == 0 {
		fmt.Println("Status: VALID")
		fmt.Println("Configuration is consistent with detected role.")
	} else {
		fmt.Println("Status: ISSUES DETECTED")
		fmt.Println()
		for _, issue := range validation.Issues {
			var prefix string
			switch issue.Severity {
			case "CRITICAL":
				prefix = "CRITICAL"
			case "WARNING":
				prefix = "WARNING"
			default:
				prefix = "INFO"
			}
			fmt.Printf("[%s] %s\n", prefix, issue.Message)
			fmt.Printf("  Field:    %s\n", issue.Field)
			fmt.Printf("  Expected: %s\n", issue.Expected)
			fmt.Printf("  Actual:   %s\n", issue.Actual)
			fmt.Println()
		}

		if len(validation.Suggestions) > 0 {
			fmt.Println("Suggestions:")
			for _, s := range validation.Suggestions {
				fmt.Printf("  - %s\n", s)
			}
		}
	}

	// Exit with error code if there are critical issues
	for _, issue := range validation.Issues {
		if issue.Severity == "CRITICAL" {
			os.Exit(1)
		}
	}
}

func runNodeReset(cmd *cobra.Command, args []string) {
	home, _ := cmd.Flags().GetString("home")
	preserveKeys, _ := cmd.Flags().GetBool("preserve-keys")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Refuse to run as root
	if os.Geteuid() == 0 {
		fmt.Fprintf(os.Stderr, "Error: monoctl must not be run as root\n")
		fmt.Fprintf(os.Stderr, "Run as the user that owns the node home directory.\n")
		os.Exit(1)
	}

	// Default home
	if home == "" {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, ".monod")
	} else if strings.HasPrefix(home, "~") {
		homeDir, _ := os.UserHomeDir()
		home = filepath.Join(homeDir, strings.TrimPrefix(home, "~"))
	}
	home, _ = filepath.Abs(home)

	// Check that home directory exists
	if _, err := os.Stat(home); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: node home directory does not exist: %s\n", home)
		fmt.Fprintf(os.Stderr, "Nothing to reset.\n")
		os.Exit(1)
	}

	// Verify this looks like a monod home directory
	configDir := filepath.Join(home, "config")
	dataDir := filepath.Join(home, "data")

	hasConfig := false
	hasData := false
	if _, err := os.Stat(configDir); err == nil {
		hasConfig = true
	}
	if _, err := os.Stat(dataDir); err == nil {
		hasData = true
	}

	if !hasConfig && !hasData {
		fmt.Fprintf(os.Stderr, "Error: %s does not look like a monod home directory\n", home)
		fmt.Fprintf(os.Stderr, "Expected to find config/ or data/ subdirectory.\n")
		os.Exit(1)
	}

	fmt.Println("Node Reset")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Home: %s\n", home)
	fmt.Println()

	// List what will be deleted
	fmt.Println("The following will be DELETED:")
	fmt.Printf("  - %s/* (blockchain data)\n", dataDir)
	fmt.Printf("  - %s/config.toml\n", home)
	fmt.Printf("  - %s/app.toml\n", home)
	fmt.Printf("  - %s/client.toml\n", home)
	fmt.Printf("  - %s/genesis.json\n", home)
	fmt.Printf("  - %s/addrbook.json\n", home)

	if preserveKeys {
		fmt.Println()
		fmt.Println("The following will be PRESERVED:")
		fmt.Printf("  - %s/config/node_key.json\n", home)
		fmt.Printf("  - %s/config/priv_validator_key.json\n", home)
	} else {
		fmt.Printf("  - %s/config/node_key.json\n", home)
		fmt.Printf("  - %s/config/priv_validator_key.json\n", home)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("(DRY RUN - no changes will be made)")
		return
	}

	// Confirmation
	if !force {
		fmt.Print("This action is DESTRUCTIVE and cannot be undone.\n")
		fmt.Print("Type 'yes' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)
		if response != "yes" {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
	}

	fmt.Println()
	fmt.Println("Resetting node...")

	// Step 1: Stop monod service
	fmt.Print("[1/5] Stopping monod service... ")
	stopCmd := exec.Command("systemctl", "--user", "stop", "monod")
	if err := stopCmd.Run(); err != nil {
		// Try system-level service
		stopCmd = exec.Command("sudo", "systemctl", "stop", "monod")
		stopCmd.Run() // Ignore errors - service might not exist
	}
	// Also kill any running monod processes
	exec.Command("pkill", "-f", "monod start").Run()
	time.Sleep(time.Second)
	fmt.Println("done")

	// Step 2: Backup keys if preserving
	var nodeKeyBackup, privValKeyBackup []byte
	if preserveKeys {
		fmt.Print("[2/5] Backing up keys... ")
		nodeKeyPath := filepath.Join(configDir, "node_key.json")
		privValKeyPath := filepath.Join(configDir, "priv_validator_key.json")

		if data, err := os.ReadFile(nodeKeyPath); err == nil {
			nodeKeyBackup = data
		}
		if data, err := os.ReadFile(privValKeyPath); err == nil {
			privValKeyBackup = data
		}
		fmt.Println("done")
	} else {
		fmt.Println("[2/5] Not preserving keys")
	}

	// Step 3: Delete everything
	fmt.Print("[3/5] Deleting node data... ")

	// Delete ENTIRE data directory (not just contents) - this ensures all *.db directories are removed
	if err := os.RemoveAll(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: failed to remove data dir: %v\n", err)
		os.Exit(1)
	}

	// Delete config files
	filesToDelete := []string{
		filepath.Join(configDir, "config.toml"),
		filepath.Join(configDir, "app.toml"),
		filepath.Join(configDir, "client.toml"),
		filepath.Join(configDir, "genesis.json"),
		filepath.Join(configDir, "addrbook.json"),
		filepath.Join(configDir, "config_patch.toml"),
	}

	if !preserveKeys {
		filesToDelete = append(filesToDelete,
			filepath.Join(configDir, "node_key.json"),
			filepath.Join(configDir, "priv_validator_key.json"),
		)
	}

	for _, f := range filesToDelete {
		os.Remove(f)
	}
	fmt.Println("done")

	// Step 4: Recreate directories and restore keys
	fmt.Print("[4/5] Recreating directories... ")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating data dir: %v\n", err)
		os.Exit(1)
	}

	// Create empty priv_validator_state.json
	privValState := `{"height": "0", "round": 0, "step": 0}`
	if err := os.WriteFile(filepath.Join(dataDir, "priv_validator_state.json"), []byte(privValState), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create priv_validator_state.json: %v\n", err)
	}

	// Restore keys if backed up
	if preserveKeys {
		if len(nodeKeyBackup) > 0 {
			os.WriteFile(filepath.Join(configDir, "node_key.json"), nodeKeyBackup, 0600)
		}
		if len(privValKeyBackup) > 0 {
			os.WriteFile(filepath.Join(configDir, "priv_validator_key.json"), privValKeyBackup, 0600)
		}
	}
	fmt.Println("done")

	// Step 5: Verify reset was successful
	fmt.Print("[5/5] Verifying reset... ")

	// Check that data directory is clean (only priv_validator_state.json should exist)
	dirtyFiles := []string{"state.db", "application.db", "blockstore.db", "tx_index.db", "evidence.db"}
	for _, df := range dirtyFiles {
		checkPath := filepath.Join(dataDir, df)
		if _, err := os.Stat(checkPath); err == nil {
			fmt.Fprintf(os.Stderr, "\nError: reset verification failed - %s still exists\n", checkPath)
			fmt.Fprintf(os.Stderr, "Data directory was not properly cleaned.\n")
			os.Exit(1)
		}
	}

	// Check that addrbook.json is removed
	addrbookPath := filepath.Join(configDir, "addrbook.json")
	if _, err := os.Stat(addrbookPath); err == nil {
		fmt.Fprintf(os.Stderr, "\nError: reset verification failed - addrbook.json still exists\n")
		os.Exit(1)
	}

	fmt.Println("done")

	fmt.Println()
	fmt.Println("Reset complete!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  monoctl join --network <network> --home %s\n", home)
}

// =============================================================================
// Config Doctor Command - Drift Detection
// =============================================================================

func runConfigDoctor(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")

	// Default home directory
	if home == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not determine home directory: %v\n", err)
			os.Exit(1)
		}
		home = filepath.Join(homeDir, ".monod")
	}

	// Parse network name
	networkName, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Fetch canonical config
	network, err := core.GetNetworkFromCanonical(networkName, "main")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching canonical config: %v\n", err)
		os.Exit(1)
	}

	// Create drift config from canonical network
	driftConfig := &core.DriftConfig{
		CosmosChainID:  network.ChainID,
		EVMChainID:     network.EVMChainID,
		Seeds:          []string{}, // Seeds come from canonical config
		BootstrapPeers: []string{}, // Peers come from canonical config
	}

	// Detect drift
	results, err := core.DetectDrift(home, driftConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting drift: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		// JSON output
		type driftOutput struct {
			Network     string             `json:"network"`
			Home        string             `json:"home"`
			DriftCount  int                `json:"drift_count"`
			HasCritical bool               `json:"has_critical"`
			Results     []core.DriftResult `json:"results"`
		}
		out := driftOutput{
			Network:     string(networkName),
			Home:        home,
			DriftCount:  len(results),
			HasCritical: core.HasCriticalDrift(results),
			Results:     results,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		if core.HasCriticalDrift(results) {
			os.Exit(1)
		}
		return
	}

	// Human-readable output
	fmt.Printf("Network: %s\n", networkName)
	fmt.Printf("Home: %s\n", home)
	fmt.Println()

	if len(results) == 0 {
		fmt.Println("No drift detected. Configuration matches canonical values.")
		return
	}

	fmt.Println(core.FormatDriftReport(results))

	if core.HasCriticalDrift(results) {
		fmt.Println()
		fmt.Printf("Run: monoctl config repair --network %s --home %s\n", networkStr, home)
		os.Exit(1)
	}
}

// =============================================================================
// Config Repair Command - Drift Repair
// =============================================================================

func runConfigRepair(cmd *cobra.Command, args []string) {
	networkStr, _ := cmd.Flags().GetString("network")
	home, _ := cmd.Flags().GetString("home")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Default home directory
	if home == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not determine home directory: %v\n", err)
			os.Exit(1)
		}
		home = filepath.Join(homeDir, ".monod")
	}

	// Parse network name
	networkName, err := core.ParseNetworkName(networkStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Fetch canonical config
	network, err := core.GetNetworkFromCanonical(networkName, "main")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching canonical config: %v\n", err)
		os.Exit(1)
	}

	// Create drift config from canonical network
	driftConfig := &core.DriftConfig{
		CosmosChainID:  network.ChainID,
		EVMChainID:     network.EVMChainID,
		Seeds:          []string{}, // Seeds come from canonical config
		BootstrapPeers: []string{}, // Peers come from canonical config
	}

	// Perform repair
	results, err := core.Repair(home, driftConfig, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error repairing configuration: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		// JSON output
		type repairOutput struct {
			Network string              `json:"network"`
			Home    string              `json:"home"`
			DryRun  bool                `json:"dry_run"`
			Results []core.RepairResult `json:"results"`
		}
		out := repairOutput{
			Network: string(networkName),
			Home:    home,
			DryRun:  dryRun,
			Results: results,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return
	}

	// Human-readable output
	fmt.Println(core.FormatRepairReport(results, dryRun))

	if !dryRun && len(results) > 0 {
		fmt.Println()
		fmt.Printf("Run: monoctl config doctor --network %s --home %s\n", networkStr, home)
		fmt.Println("to verify the repair was successful.")
	}
}

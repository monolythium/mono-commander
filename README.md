# Mono Commander

A TUI-first command-line tool for installing and operating Monolythium nodes across networks.

## Features

- **TUI-first**: Interactive terminal UI with keyboard navigation
- **CLI mode**: Non-interactive commands for CI/automation
- **Multi-network**: Support for Localnet, Sprintnet, Testnet, and Mainnet
- **Dry-run support**: Preview changes before applying them
- **Systemd integration**: Generate systemd unit files (with optional Cosmovisor)

## Supported Networks

| Network    | Chain ID       | EVM Chain ID | EVM Hex  |
|------------|----------------|--------------|----------|
| Localnet   | mono-local-1   | 262145       | 0x40001  |
| Sprintnet  | mono-sprint-1  | 262146       | 0x40002  |
| Testnet    | mono-test-1    | 262147       | 0x40003  |
| Mainnet    | mono-1         | 262148       | 0x40004  |

## Installation

```bash
go install github.com/monolythium/mono-commander/cmd/monoctl@latest
```

Or build from source:

```bash
git clone https://github.com/monolythium/mono-commander.git
cd mono-commander
go build -o monoctl ./cmd/monoctl
```

## Usage

### Interactive TUI

Launch the interactive terminal UI:

```bash
monoctl
```

### CLI Commands

#### List Networks

```bash
monoctl networks list
monoctl networks list --json
```

#### Join a Network

Download genesis and configure peers for a network:

```bash
# Dry run (preview only)
monoctl join --network Sprintnet --home ~/.monod --dry-run

# Actual join with SHA verification
monoctl join \
  --network Sprintnet \
  --home ~/.monod \
  --genesis-sha256 <expected-sha256>
```

#### Update Peers

Update peer list from the network registry:

```bash
monoctl peers update --network Sprintnet --home ~/.monod --dry-run
```

#### Install Systemd Service

Generate a systemd unit file:

```bash
# Preview the unit file
monoctl systemd install \
  --network Sprintnet \
  --home ~/.monod \
  --user monod \
  --dry-run

# With Cosmovisor
monoctl systemd install \
  --network Mainnet \
  --home ~/.monod \
  --user monod \
  --cosmovisor \
  --dry-run
```

### Mesh/Rosetta API Sidecar

The Mesh/Rosetta API is a compatibility layer for blockchain integrations. It is optional but recommended for RPC/indexer nodes.

#### Install Mesh Binary

```bash
# Dry-run (preview only)
monoctl mesh install \
  --url https://example.com/mono-mesh-rosetta \
  --sha256 <expected-sha256> \
  --version v0.1.0 \
  --dry-run

# Actual install
monoctl mesh install \
  --url https://example.com/mono-mesh-rosetta \
  --sha256 <expected-sha256> \
  --version v0.1.0
```

#### Enable Mesh Service

```bash
# Dry-run
monoctl mesh enable --network Sprintnet --dry-run

# Actual enable
monoctl mesh enable --network Sprintnet
```

#### Check Mesh Status

```bash
monoctl mesh status --network Sprintnet
monoctl mesh status --network Sprintnet --json
```

#### View Mesh Logs

```bash
monoctl mesh logs --network Sprintnet --lines 100
monoctl mesh logs --network Sprintnet --follow
```

#### Disable Mesh Service

```bash
monoctl mesh disable --network Sprintnet --dry-run
monoctl mesh disable --network Sprintnet
```

#### Mesh Network Ports

| Network    | Mesh Port |
|------------|-----------|
| Localnet   | 8080      |
| Sprintnet  | 8081      |
| Testnet    | 8082      |
| Mainnet    | 8083      |

### Self-Update

monoctl can update itself from GitHub Releases with checksum verification.

#### Check for Updates

```bash
# Check if an update is available
monoctl update check

# JSON output for automation
monoctl update check --json
```

#### Apply Update

```bash
# Interactive update with confirmation
monoctl update apply

# Skip confirmation (for CI/automation)
monoctl update apply --yes

# Preview without making changes
monoctl update apply --dry-run

# Skip checksum verification (not recommended)
monoctl update apply --insecure
```

#### Update via TUI

In the interactive TUI, navigate to the **Update** tab and press `u` to apply an available update.

#### Safety Features

- **Checksum verification**: Downloads are verified against SHA256 checksums from the release
- **Safe binary swap**: Downloads to temp location, verifies, then atomically swaps
- **Backup retention**: Old binary is preserved with `.backup` suffix
- **Dry-run support**: Preview changes before applying

## Safety Notes

### Security

- **No secrets stored**: This tool never stores or logs mnemonics, private keys, or tokens
- **No key generation**: Key management is handled separately by the node binary
- **Dry-run by default recommended**: Always use `--dry-run` first to preview changes

### Operations

- **No rollback support**: In case of issues, the recovery path is:
  HALT → PATCH → UPGRADE → RESTART
- **Systemd-first**: Node operation uses systemd/cosmovisor (no Docker runtime)
- **User confirmation required**: Systemd service is not auto-enabled

## Architecture

```
mono-commander/
├── cmd/monoctl/          # CLI entry point
├── internal/
│   ├── core/             # Core logic (networks, genesis, peers, config)
│   ├── tui/              # Bubble Tea TUI
│   ├── os/               # Systemd/cosmovisor helpers
│   ├── net/              # HTTP fetcher
│   ├── mesh/             # Mesh/Rosetta API sidecar management
│   ├── update/           # Self-update (GitHub releases, checksums, safe swap)
│   ├── rpc/              # RPC helpers (future)
│   └── logs/             # Log helpers
└── testdata/             # Test fixtures
```

## Development

### Run Tests

```bash
go test ./...
```

### Build

```bash
go build -o monoctl ./cmd/monoctl
```

## License

Apache 2.0

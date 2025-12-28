# Mono Commander

A TUI-first command-line tool for installing and operating Monolythium nodes across networks.

## Quick Start

### Prerequisites

**Go 1.23+** is required. If you don't have Go installed:

```bash
# macOS (Homebrew)
brew install go

# Ubuntu/Debian
sudo apt update && sudo apt install -y golang-go

# Or download from https://go.dev/dl/
```

Verify installation:
```bash
go version
# Should show: go version go1.23.x ...
```

### Install Mono Commander

**Option 1: One-liner install (recommended)**
```bash
go install github.com/monolythium/mono-commander/cmd/monoctl@latest
```

**Option 2: Build from source**
```bash
git clone https://github.com/monolythium/mono-commander.git
cd mono-commander
go build -o monoctl ./cmd/monoctl
sudo mv monoctl /usr/local/bin/  # Optional: add to PATH
```

### Launch the TUI

```bash
monoctl
```

That's it! The interactive TUI will guide you through node setup.

## Features

- **Premium TUI**: Interactive terminal UI with gradient borders, dark theme, mouse support
- **CLI mode**: Non-interactive commands for CI/automation
- **Multi-network**: Support for Localnet, Sprintnet, Testnet, and Mainnet
- **Self-updating**: Auto-update from GitHub Releases with checksum verification
- **Health monitoring**: Real-time node status, RPC checks, validator health
- **Log streaming**: Live log viewing with filtering
- **Dry-run support**: Preview changes before applying them
- **Systemd integration**: Generate systemd unit files (with optional Cosmovisor)

## Supported Networks

| Network    | Chain ID       | EVM Chain ID | EVM Hex  |
|------------|----------------|--------------|----------|
| Localnet   | mono-local-1   | 262145       | 0x40001  |
| Sprintnet  | mono-sprint-1  | 262146       | 0x40002  |
| Testnet    | mono-test-1    | 262147       | 0x40003  |
| Mainnet    | mono-1         | 262148       | 0x40004  |

## TUI Navigation

| Key | Action |
|-----|--------|
| `Tab` / `←` `→` | Switch tabs |
| `Enter` | Select / Confirm |
| `Esc` | Go back / Cancel |
| `r` | Refresh current view |
| `n` | Change network (Dashboard) |
| `q` | Quit |

**Mouse support**: Click tabs to switch, scroll wheel for viewports.

## CLI Commands

### List Networks

```bash
monoctl networks list
monoctl networks list --json
```

### Join a Network

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

### Update Peers

Update peer list from the network registry:

```bash
monoctl peers update --network Sprintnet --home ~/.monod --dry-run
```

### Install Systemd Service

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

### Node Status

```bash
monoctl status --network Sprintnet
monoctl status --network Sprintnet --json
```

### Health Checks

```bash
monoctl health --network Sprintnet
monoctl health --network Sprintnet --json
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

## Terminal Requirements

For the best experience:
- **Truecolor terminal** (iTerm2, Alacritty, Kitty, Windows Terminal) for gradient borders
- Falls back gracefully to 256-color mode

To verify truecolor support:
```bash
echo $COLORTERM
# Should show: truecolor or 24bit
```

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
│   ├── rpc/              # RPC helpers (Comet, Cosmos, EVM)
│   └── logs/             # Log streaming helpers
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

### Build with Version

```bash
go build -ldflags "-X github.com/monolythium/mono-commander/internal/tui.Version=v1.0.0" -o monoctl ./cmd/monoctl
```

## Troubleshooting

**RPC unreachable?**
- Check if the node is running: `systemctl status monod`
- Verify the node is listening on expected ports

**Wrong chain-id?**
- Ensure you're connected to the correct network
- Check genesis.json matches the expected network

**Ports in use?**
- Stop conflicting services: `lsof -i :26657`
- Use different ports in config.toml

**Systemd not present?**
- Systemd is required for service management on Linux
- On macOS, use launchd or run manually

## License

Mono Commander is source-available under the [Business Source License 1.1](LICENSE).

- **Non-production use** is permitted (evaluation, personal, educational, internal)
- **Production use** as part of a competing product or service requires a commercial license
- On the Change Date (2029-01-01), the license converts to Apache-2.0

See [LICENSE](LICENSE) for full terms.

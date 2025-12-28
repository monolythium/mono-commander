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
│   ├── rpc/              # RPC helpers (future)
│   └── logs/             # Log helpers (future)
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

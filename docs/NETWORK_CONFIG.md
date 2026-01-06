# Network Configuration Architecture

This document describes the canonical network configuration architecture introduced to prevent configuration drift and ensure consistent node setup across the Monolythium network.

## Overview

The architecture follows a **single-writer model** where:
1. `monolythium/networks` repo is the **sole source of truth** for network configs
2. `monoctl` is the **exclusive writer** of TOML configuration files
3. `monod` **validates** configuration at startup (fail loud)

This prevents the root cause of consensus failures due to misconfigured nodes.

## Configuration Sources

### Canonical Source: `monolythium/networks`

The `monolythium/networks` repository contains:
- JSON Schema for validation (`schema/network.schema.json`)
- Network configs (`networks/{localnet,sprintnet,testnet,mainnet}.json`)
- Registry index (`index.json`)
- CI validation workflow

**GitHub URL**: `https://github.com/monolythium/networks`

### Network Config Fields

Each network config contains:

| Field | Type | Description |
|-------|------|-------------|
| `network_name` | string | Human-readable name (Sprintnet, Testnet, etc.) |
| `cosmos_chain_id` | string | Cosmos chain identifier (mono-sprint-1) |
| `evm_chain_id` | uint64 | EIP-155 chain ID (262146) |
| `evm_chain_id_hex` | string | Hex representation (0x40002) |
| `genesis_url` | string | URL to canonical genesis.json |
| `genesis_sha256` | string | SHA-256 hash for verification |
| `seeds` | []string | Seed nodes (node_id@host:port) |
| `bootstrap_peers` | []string | Bootstrap peers for deterministic sync |
| `network_status` | string | active, testing, deprecated |

## EVM Chain ID Allocation

| Network | Chain ID | Hex | Status |
|---------|----------|-----|--------|
| Localnet | 262145 | 0x40001 | Development only |
| Sprintnet | 262146 | 0x40002 | Active testnet |
| Testnet | 262147 | 0x40003 | Reserved |
| Mainnet | 262148 | 0x40004 | Reserved |

**CRITICAL INVARIANT**: Sprintnet MUST always use EVM chain ID 262146. This is enforced at multiple levels:
1. CI validation in networks repo
2. `monoctl` config verification
3. `monod` runtime assertion

## Configuration Flow

### 1. Fetch Phase

```
monoctl join --network Sprintnet
    │
    ├─> FetchNetworkConfig("sprintnet", "main")
    │       │
    │       └─> GET https://raw.githubusercontent.com/monolythium/networks/main/networks/sprintnet.json
    │
    ├─> VerifyNetworkConfig(config)
    │       │
    │       ├─> Check EVM chain ID matches expected
    │       └─> Validate genesis SHA256 format
    │
    └─> ValidateNotLocalnetLeak(config)
            │
            └─> FATAL if non-Localnet has EVM chain ID 262145
```

### 2. Cache Phase

Configs are cached locally for offline operation:

```
~/.mono-commander/networks/
  └── Sprintnet/
      └── main/
          ├── config.json    # Full network config
          └── meta.json      # Cache metadata (timestamp, ref)
```

### 3. Write Phase

monoctl writes all TOML files from the canonical config:

| File | Field | Source |
|------|-------|--------|
| `client.toml` | chain-id | `config.CosmosChainID` |
| `app.toml` | evm-chain-id | `config.EVMChainID` |
| `config.toml` | seeds | `config.Seeds` |
| `config.toml` | persistent_peers | `config.BootstrapPeers` |

### 4. Validate Phase (Runtime)

monod validates at startup:

```go
func validateChainConfig(chainID string, evmChainID uint64) error {
    switch {
    case strings.HasPrefix(chainID, "mono-sprint"):
        if evmChainID != 262146 {
            return fmt.Errorf("Sprintnet requires evm-chain-id=262146, got %d", evmChainID)
        }
    // ... other networks
    }
    return nil
}
```

## Drift Detection

Configuration drift occurs when on-disk files differ from canonical values.

### Severity Levels

- **CRITICAL**: Causes consensus failure
  - Wrong `chain-id` in client.toml
  - Wrong `evm-chain-id` in app.toml

- **WARNING**: Causes connectivity issues
  - Incorrect seeds
  - Incorrect persistent_peers

### Detection

```bash
monoctl doctor --network Sprintnet --home ~/.monod
```

Output:
```
Network: Sprintnet
Config Pin: main (e1ba912)

DRIFT DETECTED:
  [CRITICAL] app.toml evm-chain-id: expected '262146', got '262145'
  [WARNING] config.toml seeds: expected 2 entries, got 0

Run: monoctl repair --network Sprintnet to fix critical issues
```

### Repair

```bash
# Dry run (preview changes)
monoctl repair --network Sprintnet --home ~/.monod --dry-run

# Apply repairs
monoctl repair --network Sprintnet --home ~/.monod
```

## Localnet Isolation

Localnet (EVM chain ID 262145) is isolated from production networks:

1. **No fetch**: Localnet config is embedded, never fetched from remote
2. **Leak prevention**: `ValidateNotLocalnetLeak()` blocks 262145 on non-Localnet
3. **No propagation**: Localnet values cannot "leak" into production configs

## Best Practices

### For Operators

1. **Never manually edit** `chain-id` or `evm-chain-id` fields
2. Use `monoctl join` for initial setup
3. Use `monoctl doctor` to check for drift
4. Use `monoctl repair` to fix drift
5. Run `monoctl doctor` after any manual config changes

### For Development

1. Use Localnet for local testing (262145)
2. Never hardcode EVM chain IDs in application code
3. Always read from canonical config

## Troubleshooting

### "FATAL: Localnet EVM chain ID detected for Sprintnet"

The node is misconfigured with Localnet's EVM chain ID (262145) instead of Sprintnet's (262146).

**Fix**:
```bash
monoctl repair --network Sprintnet --home ~/.monod
```

### "invalid chain-id on InitChain"

The `chain-id` in client.toml doesn't match the genesis.

**Fix**:
```bash
monoctl doctor --network Sprintnet --home ~/.monod
monoctl repair --network Sprintnet --home ~/.monod
```

### "AppHash mismatch"

Nodes computed different state, likely due to EVM chain ID mismatch.

**Diagnosis**:
```bash
# Check your EVM chain ID
grep evm-chain-id ~/.monod/config/app.toml

# Should show: evm-chain-id = 262146 for Sprintnet
```

**Fix**:
1. Stop the node
2. Run `monoctl repair --network Sprintnet --home ~/.monod`
3. Wipe data: `monoctl reset --home ~/.monod`
4. Restart and sync from genesis

## API Reference

### Core Functions

```go
// Fetch config from canonical source with cache fallback
func GetNetworkFromCanonical(name NetworkName, ref string) (Network, error)

// Validate config doesn't use Localnet's EVM chain ID
func ValidateNotLocalnetLeak(config *NetworkConfig) error

// Detect configuration drift
func DetectDrift(home string, config *DriftConfig) ([]DriftResult, error)

// Repair configuration drift
func Repair(home string, config *DriftConfig, dryRun bool) ([]RepairResult, error)
```

### Constants

```go
// LocalnetEVMChainID is reserved for development only
const LocalnetEVMChainID = 262145
```

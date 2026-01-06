# W4 Implementation Summary: Drift Detection and Repair

## Overview

This implementation adds drift detection and repair functionality to monoctl, enabling operators to:
1. Detect configuration drift between on-disk files and canonical network settings
2. Repair configuration drift automatically or via dry-run preview
3. Ensure nodes maintain correct configuration for consensus and connectivity

## Files Created

### Core Implementation

#### 1. `/internal/core/drift.go`
**Purpose**: Drift detection logic

**Key Components**:
- `DriftSeverity` enum (CRITICAL, WARNING, INFO)
- `DriftResult` struct - represents a single configuration drift
- `DriftConfig` struct - holds expected configuration values
- `DetectDrift(home, config)` - detects all configuration drift
- `HasCriticalDrift(drifts)` - checks for critical issues
- `FormatDriftReport(drifts)` - formats drift results for display
- `ParsePeer(s)` - public wrapper for parsing peer strings

**What it checks**:
- client.toml: chain-id (CRITICAL)
- app.toml: evm-chain-id (CRITICAL)
- config.toml: seeds (WARNING)
- config.toml: persistent_peers (WARNING)

#### 2. `/internal/core/repair.go`
**Purpose**: Configuration repair logic

**Key Components**:
- `RepairResult` struct - describes what was repaired
- `Repair(home, config, dryRun)` - repairs configuration drift
- `FormatRepairReport(results, dryRun)` - formats repair results

**What it repairs**:
- Sets correct chain-id in client.toml
- Sets correct evm-chain-id in app.toml
- Updates seeds and persistent_peers in config.toml

### Tests

#### 3. `/internal/core/drift_test.go`
**Purpose**: Unit tests for drift detection

**Test Coverage**:
- `TestParsePeer` - validates peer string parsing
- `TestDetectDrift` - validates drift detection with/without drift
- `TestFormatDriftReport` - validates report formatting

#### 4. `/internal/core/repair_test.go`
**Purpose**: Unit tests for configuration repair

**Test Coverage**:
- `TestRepair` - validates dry-run and actual repair
- `TestFormatRepairReport` - validates report formatting

### Documentation

#### 5. `/docs/DRIFT_DETECTION.md`
**Purpose**: User-facing documentation

**Contents**:
- Overview of drift detection concept
- Core functionality examples
- Severity level explanations
- Integration examples
- Best practices
- Future enhancements

#### 6. `/docs/W4_IMPLEMENTATION_SUMMARY.md`
**Purpose**: Implementation documentation (this file)

## Integration with Existing Code

### Dependencies
The implementation leverages existing core functionality:
- `GetConfigValue()` - reads TOML config values
- `SetClientChainID()` - sets chain-id in client.toml
- `SetEVMChainID()` - sets evm-chain-id in app.toml
- `ApplyConfigPatch()` - applies p2p configuration changes
- `Peer` struct and related functions from peers.go

### Enhancements to Existing Code
Added helper function to peers.go:
- `PeersToStringSlice()` - converts []Peer to []string for drift detection

### Design Decisions

1. **DriftConfig struct**: Created a separate config struct to avoid coupling with Network or PeersRegistry, making it easier to construct for testing and different use cases.

2. **Severity levels**: Classified drift by impact:
   - CRITICAL: Causes node failures or consensus issues
   - WARNING: Causes connectivity or sync issues
   - INFO: Reserved for future non-critical checks

3. **Dry-run support**: All repair operations support dry-run mode for safe preview.

4. **Public ParsePeer**: Exported a public version of the internal parsePeerString function for reusability.

## Next Steps: CLI Integration

The core functionality is now ready for CLI integration. Suggested commands:

### Doctor Command
```bash
monoctl doctor --network Sprintnet --home ~/.monod
```
Detects and reports configuration drift.

### Repair Command
```bash
# Dry run
monoctl repair --network Sprintnet --home ~/.monod --dry-run

# Apply repairs
monoctl repair --network Sprintnet --home ~/.monod
```
Repairs configuration drift.

### Example CLI Implementation
```go
doctorCmd := &cobra.Command{
    Use:   "doctor",
    Short: "Check configuration for drift",
    Run: func(cmd *cobra.Command, args []string) {
        network, _ := core.GetNetwork(networkName)

        // Fetch peers registry
        fetcher := &net.HTTPFetcher{}
        peersData, _ := fetcher.Fetch(network.PeersURL)
        reg, _ := core.ParsePeersRegistry(peersData)

        // Build drift config
        config := &core.DriftConfig{
            CosmosChainID:  network.ChainID,
            EVMChainID:     network.EVMChainID,
            Seeds:          core.PeersToStringSlice(reg.Seeds),
            BootstrapPeers: core.PeersToStringSlice(reg.BootstrapPeers),
        }

        // Detect drift
        drifts, _ := core.DetectDrift(home, config)
        fmt.Println(core.FormatDriftReport(drifts))
    },
}
```

## Testing

All code compiles successfully:
```bash
cd /Users/nayiemwillems/workspace/monolythium/mono-commander
GOTOOLCHAIN=auto go build ./internal/core
```

Unit tests are provided for all major functionality.

## Compatibility

- **Go version**: Compatible with go 1.24.0+ (as required by go.mod)
- **Dependencies**: Uses only existing dependencies (no new imports)
- **Breaking changes**: None (additive only)

## Future Enhancements

Potential improvements for later phases:
1. Check additional config parameters (pruning, state sync, gas prices)
2. Validate genesis.json against canonical SHA256
3. Check for stale addrbook.json
4. Verify port configurations match expected values
5. Integration with monitoring/alerting systems
6. Automated drift repair on network upgrades
7. Historical drift tracking and reporting

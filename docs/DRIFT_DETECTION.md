# Drift Detection and Repair

The drift detection and repair functionality helps ensure that node configurations match the canonical network settings defined in the mono-core-peers registry.

## Overview

**Drift** occurs when on-disk configuration files (client.toml, app.toml, config.toml) differ from the canonical network settings. This can happen due to:
- Manual configuration changes
- Network upgrades with new settings
- Migration from different networks
- Incomplete join operations

## Core Functionality

### Drift Detection

The `DetectDrift` function compares on-disk configuration against expected canonical values:

```go
// Example usage
config := &core.DriftConfig{
    CosmosChainID:  "mono-sprint-1",
    EVMChainID:     262146,
    Seeds:          []string{"node1@seed1.example.com:26656"},
    BootstrapPeers: []string{},
}

drifts, err := core.DetectDrift("/path/to/.monod", config)
if err != nil {
    // Handle error
}

// Check for critical issues
if core.HasCriticalDrift(drifts) {
    fmt.Println("Critical configuration drift detected!")
}

// Display drift report
fmt.Println(core.FormatDriftReport(drifts))
```

### Drift Severity Levels

- **CRITICAL**: Issues that will cause consensus failures or node startup problems
  - Wrong Cosmos chain-id (client.toml)
  - Wrong EVM chain-id (app.toml)

- **WARNING**: Issues that may cause connectivity or sync problems
  - Incorrect seeds configuration
  - Incorrect persistent_peers configuration

- **INFO**: Non-critical differences (future use)

### Repair Functionality

The `Repair` function applies canonical configuration to fix detected drift:

```go
// Example usage
config := &core.DriftConfig{
    CosmosChainID:  "mono-sprint-1",
    EVMChainID:     262146,
    Seeds:          []string{"node1@seed1.example.com:26656"},
    BootstrapPeers: []string{},
}

// Dry run first (recommended)
results, err := core.Repair("/path/to/.monod", config, true)
if err != nil {
    // Handle error
}

fmt.Println(core.FormatRepairReport(results, true))

// If dry run looks good, apply changes
results, err = core.Repair("/path/to/.monod", config, false)
if err != nil {
    // Handle error
}

fmt.Println(core.FormatRepairReport(results, false))
```

## Configuration Files

### client.toml
- **chain-id**: Must match the Cosmos chain ID for the network
- **Severity**: CRITICAL (wrong chain-id causes "invalid chain-id on InitChain" errors)

### app.toml
- **evm-chain-id**: Must match the EVM chain ID for the network
- **Severity**: CRITICAL (wrong evm-chain-id causes AppHash mismatches)

### config.toml
- **seeds**: Comma-separated list of seed nodes (node_id@host:port)
- **persistent_peers**: Comma-separated list of persistent peers
- **Severity**: WARNING (affects connectivity but not consensus)

## Integration Example

Here's how to integrate drift detection into a "doctor" command:

```go
func runDoctor(home string, networkName core.NetworkName) error {
    // Get network configuration
    network, err := core.GetNetwork(networkName)
    if err != nil {
        return err
    }

    // Fetch peers registry
    fetcher := &net.HTTPFetcher{}
    peersData, err := fetcher.Fetch(network.PeersURL)
    if err != nil {
        return err
    }

    reg, err := core.ParsePeersRegistry(peersData)
    if err != nil {
        return err
    }

    // Build drift config
    driftConfig := &core.DriftConfig{
        CosmosChainID:  network.ChainID,
        EVMChainID:     network.EVMChainID,
        Seeds:          core.PeersToStringSlice(reg.Seeds),
        BootstrapPeers: core.PeersToStringSlice(reg.BootstrapPeers),
    }

    // Detect drift
    drifts, err := core.DetectDrift(home, driftConfig)
    if err != nil {
        return err
    }

    // Display results
    fmt.Println(core.FormatDriftReport(drifts))

    return nil
}
```

## Helper Functions

### ParsePeer

The `ParsePeer` function parses a peer string in the format "node_id@host:port":

```go
peer, err := core.ParsePeer("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0@seed1.example.com:26656")
if err != nil {
    // Handle invalid peer format
}

fmt.Printf("Node ID: %s\n", peer.NodeID)
fmt.Printf("Address: %s\n", peer.Address)
fmt.Printf("Port: %d\n", peer.Port)
```

## Best Practices

1. **Always use dry-run first**: Preview changes before applying them
2. **Backup configs**: Keep backups of working configurations
3. **Run doctor regularly**: Periodically check for drift
4. **Fix critical drift immediately**: Critical drift can cause node failures
5. **Verify after repair**: Run drift detection again after repair to confirm fixes

## Future Enhancements

Potential future additions:
- Detection of additional config parameters (gas prices, pruning settings, etc.)
- Historical drift tracking
- Automated drift repair on network upgrades
- Integration with monitoring/alerting systems
- Support for custom validation rules

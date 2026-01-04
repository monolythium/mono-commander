# Docker Mode - Monolythium Node Deployment

## Overview

Docker mode provides a containerized deployment option for Monolythium nodes as an alternative to the host-native systemd deployment. This mode is ideal for:

- Development and testing environments
- Multi-network node operators
- Users who prefer container-based deployments
- Systems without systemd support

## Architecture

### What Runs in Containers

- **monod binary**: The Monolythium node daemon
- **Process**: Single container running `monod start --home /root/.monod`
- **Network mode**: Bridge (default) or host (for performance)

### Persistent Volumes

All critical data is stored in a volume mounted at `~/.monod`:

```
~/.monod/
├── config/
│   ├── genesis.json         # Network genesis
│   ├── config.toml          # CometBFT config
│   ├── app.toml             # Cosmos SDK config
│   └── priv_validator_key.json  # Consensus key (CRITICAL)
├── data/
│   └── priv_validator_state.json  # Validator state
└── keyring-file/            # Optional: wallet keys
```

### Exposed Ports

| Service       | Container Port | Host Port | Protocol |
|---------------|----------------|-----------|----------|
| P2P           | 26656          | 26656     | TCP      |
| RPC           | 26657          | 26657     | TCP      |
| Cosmos REST   | 1317           | 1317      | TCP      |
| gRPC          | 9090           | 9090      | TCP      |
| EVM JSON-RPC  | 8545           | 8545      | TCP/HTTP |
| EVM WebSocket | 8546           | 8546      | TCP/WS   |

## Command Reference

### Initialize Docker Environment

```bash
monoctl docker init --network Sprintnet --home ~/.monod [--dry-run]
```

Downloads genesis, configures peers, and creates `docker-compose.yml`.

### Start Container

```bash
monoctl docker up
```

### Stop Container

```bash
monoctl docker down
```

### View Logs

```bash
monoctl docker logs [--follow]
```

### Check Status

```bash
monoctl docker status
```

## Security: Keys Inside Container Volume

- Private keys stored in container volume at `/root/.monod/config/priv_validator_key.json`
- Volume is mounted from host filesystem (`~/.monod`)
- Keys managed via `docker exec`:
  ```bash
  monoctl docker exec monod keys list
  monoctl docker exec monod keys add mykey
  ```

## Comparison: Docker vs Host-Native

| Aspect              | Docker Mode                  | Host-Native (systemd)       |
|---------------------|------------------------------|-----------------------------|
| Binary Management   | Image pull                   | Direct download             |
| Process Management  | docker-compose               | systemd                     |
| Upgrades            | Image tag bump + restart     | Binary replace + restart    |
| Isolation           | Container isolation          | Direct host execution       |
| Performance         | Slight overhead              | Native performance          |
| Platform Support    | Docker available             | Linux with systemd          |

## Limitations

1. **No Cosmovisor Support**: Docker mode uses simple binary execution
2. **No Auto-Upgrades**: Requires manual image tag updates
3. **Performance Overhead**: Container networking adds latency (use host mode for validators)

// Package core provides monitoring functionality for node-monitor integration.
package core

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// MonitorConfig holds configuration for monitoring.
type MonitorConfig struct {
	APIEndpoint string // node-monitor API endpoint
	Network     string // Network name (Sprintnet, Testnet, Mainnet)
	Home        string // Node home directory
	KeysDir     string // Directory for monitor keys
}

// MonitorKeys holds the ed25519 keypair for signing heartbeats.
type MonitorKeys struct {
	NodeID     string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// MonitorStatus holds the local node status for heartbeats.
type MonitorStatus struct {
	Height             int64
	CatchingUp         bool
	EarliestBlockHeight int64
	VersionMonod       string
	VersionMonoctl     string
	ChainID            string
	EVMChainID         uint64
	PeerCount          int
}

// MonitorCapabilities holds local node capabilities.
type MonitorCapabilities struct {
	PruningMode      string
	StateSyncEnabled bool
	SnapshotsEnabled bool
	Services         ServiceFlags
	Ports            PortConfig
}

// ServiceFlags indicates which services are available locally.
type ServiceFlags struct {
	RPC    bool `json:"rpc"`
	REST   bool `json:"rest"`
	GRPC   bool `json:"grpc"`
	EVMRPC bool `json:"evm_rpc"`
}

// PortConfig holds detected port numbers.
type PortConfig struct {
	P2P    int `json:"p2p"`
	RPC    int `json:"rpc"`
	REST   int `json:"rest"`
	GRPC   int `json:"grpc"`
	EVMRPC int `json:"evm_rpc"`
}

// HeartbeatPayload is the structure sent to the node-monitor API.
type HeartbeatPayload struct {
	NodeID        string              `json:"node_id"`
	Network       string              `json:"network"`
	TimestampUnix int64               `json:"timestamp_unix"`
	Nonce         string              `json:"nonce"`
	Signature     string              `json:"signature"`
	Status        HeartbeatStatus     `json:"status"`
	Capabilities  MonitorCapabilities `json:"capabilities"`
}

// HeartbeatStatus is the status portion of the heartbeat.
type HeartbeatStatus struct {
	Height             int64  `json:"height"`
	CatchingUp         bool   `json:"catching_up"`
	EarliestBlockHeight int64 `json:"earliest_block_height"`
	VersionMonod       string `json:"version_monod"`
	VersionMonoctl     string `json:"version_monoctl"`
	ChainID            string `json:"chain_id"`
	EVMChainID         uint64 `json:"evm_chain_id"`
	PeerCount          int    `json:"peer_count,omitempty"`
}

// HeartbeatResponse is the response from the node-monitor API.
type HeartbeatResponse struct {
	Success         bool   `json:"success"`
	Health          string `json:"health"`
	LagBlocks       int64  `json:"lag_blocks"`
	CanonicalHeight int64  `json:"canonical_height"`
}

// RegistrationStartRequest is sent to begin registration.
type RegistrationStartRequest struct {
	NodeID    string `json:"node_id"`
	Network   string `json:"network"`
	Moniker   string `json:"moniker"`
	Role      string `json:"role"`
	PublicKey string `json:"public_key"`
}

// RegistrationStartResponse contains the link token.
type RegistrationStartResponse struct {
	LinkToken string    `json:"link_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DefaultMonitorAPIEndpoint returns the default node-monitor API endpoint.
// Deprecated: Use MonitorAPIEndpointForNetwork instead.
func DefaultMonitorAPIEndpoint() string {
	if endpoint := os.Getenv("NODEMON_API"); endpoint != "" {
		return endpoint
	}
	return "https://nodemon.sprintnet.mononodes.xyz"
}

// MonitorAPIEndpointForNetwork returns the node-monitor API endpoint for a specific network.
func MonitorAPIEndpointForNetwork(network string) string {
	// Allow override via environment variable
	if endpoint := os.Getenv("NODEMON_API"); endpoint != "" {
		return endpoint
	}

	// Network-specific endpoints
	switch strings.ToLower(network) {
	case "sprintnet":
		return "https://nodemon.sprintnet.mononodes.xyz"
	case "testnet":
		return "https://nodemon.testnet.mononodes.xyz"
	case "mainnet":
		return "https://nodemon.mainnet.mononodes.xyz"
	default:
		// For localnet or unknown networks, use sprintnet as fallback
		return "https://nodemon.sprintnet.mononodes.xyz"
	}
}

// GetMonitorKeysDir returns the directory for monitor keys.
func GetMonitorKeysDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mono-commander", "monitor"), nil
}

// LoadOrCreateKeys loads existing keys or creates new ones.
// home is the monod home directory for reading node_key.json.
func LoadOrCreateKeys(keysDir, home string) (*MonitorKeys, error) {
	pubKeyPath := filepath.Join(keysDir, "monitor.pub")

	// Check if keys exist
	if _, err := os.Stat(pubKeyPath); err == nil {
		return LoadKeys(keysDir, home)
	}

	// Create new keys
	return CreateKeys(keysDir, home)
}

// CreateKeys generates a new ed25519 keypair and saves it.
// home is the monod home directory for reading node_key.json.
func CreateKeys(keysDir, home string) (*MonitorKeys, error) {
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keys directory: %w", err)
	}

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Save public key (base64)
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	pubKeyPath := filepath.Join(keysDir, "monitor.pub")
	if err := os.WriteFile(pubKeyPath, []byte(pubKeyB64), 0644); err != nil {
		return nil, fmt.Errorf("failed to save public key: %w", err)
	}

	// Save private key (base64, restricted permissions)
	privKeyB64 := base64.StdEncoding.EncodeToString(privKey)
	if err := os.WriteFile(filepath.Join(keysDir, "monitor.key"), []byte(privKeyB64), 0600); err != nil {
		return nil, fmt.Errorf("failed to save private key: %w", err)
	}

	// Get node ID from local node
	nodeID, err := GetLocalNodeID(home)
	if err != nil {
		return nil, fmt.Errorf("failed to get node ID: %w", err)
	}

	return &MonitorKeys{
		NodeID:     nodeID,
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}

// LoadKeys loads existing keys from disk.
// home is the monod home directory for reading node_key.json.
func LoadKeys(keysDir, home string) (*MonitorKeys, error) {
	pubKeyPath := filepath.Join(keysDir, "monitor.pub")
	privKeyPath := filepath.Join(keysDir, "monitor.key")

	pubKeyB64, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	privKeyB64, err := os.ReadFile(privKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	pubKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(pubKeyB64)))
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	privKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(privKeyB64)))
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	nodeID, err := GetLocalNodeID(home)
	if err != nil {
		return nil, fmt.Errorf("failed to get node ID: %w", err)
	}

	return &MonitorKeys{
		NodeID:     nodeID,
		PublicKey:  ed25519.PublicKey(pubKey),
		PrivateKey: ed25519.PrivateKey(privKey),
	}, nil
}

// GetLocalNodeID retrieves the node ID from the local node.
// If home is empty, uses MONOD_HOME env var or defaults to ~/.monod.
func GetLocalNodeID(home string) (string, error) {
	// Determine home directory
	if home == "" {
		home = os.Getenv("MONOD_HOME")
		if home == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("could not determine home directory: %w", err)
			}
			home = filepath.Join(userHome, ".monod")
		}
	}

	// Try using monod tendermint show-node-id with --home flag
	cmd := exec.Command("monod", "tendermint", "show-node-id", "--home", home)
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	// Fallback: read from node_key.json and derive node ID
	nodeKeyPath := filepath.Join(home, "config", "node_key.json")
	data, err := os.ReadFile(nodeKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read node_key.json: %w", err)
	}

	var nodeKey struct {
		PrivKey struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"priv_key"`
	}
	if err := json.Unmarshal(data, &nodeKey); err != nil {
		return "", fmt.Errorf("failed to parse node_key.json: %w", err)
	}

	// Decode private key and derive node ID
	privKeyBytes, err := base64.StdEncoding.DecodeString(nodeKey.PrivKey.Value)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %w", err)
	}

	// For ed25519, public key is last 32 bytes of 64-byte private key
	if len(privKeyBytes) != 64 {
		return "", fmt.Errorf("unexpected private key length: %d", len(privKeyBytes))
	}
	pubKey := privKeyBytes[32:]

	// Node ID is first 20 bytes of SHA256 of public key, hex encoded
	hash := sha256.Sum256(pubKey)
	nodeID := hex.EncodeToString(hash[:20])

	return nodeID, nil
}

// GetLocalStatus retrieves the current node status.
func GetLocalStatus(home string, version string) (*MonitorStatus, error) {
	status := &MonitorStatus{
		VersionMonoctl: version,
	}

	// Try localhost:26657/status
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:26657/status")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local RPC: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read status response: %w", err)
	}

	var result struct {
		Result struct {
			NodeInfo struct {
				Version string `json:"version"`
			} `json:"node_info"`
			SyncInfo struct {
				LatestBlockHeight   string `json:"latest_block_height"`
				EarliestBlockHeight string `json:"earliest_block_height"`
				CatchingUp          bool   `json:"catching_up"`
			} `json:"sync_info"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	status.Height, _ = strconv.ParseInt(result.Result.SyncInfo.LatestBlockHeight, 10, 64)
	status.EarliestBlockHeight, _ = strconv.ParseInt(result.Result.SyncInfo.EarliestBlockHeight, 10, 64)
	status.CatchingUp = result.Result.SyncInfo.CatchingUp
	status.VersionMonod = result.Result.NodeInfo.Version

	// Read chain_id from client.toml
	clientTomlPath := filepath.Join(home, "config", "client.toml")
	if chainID, err := readTomlValue(clientTomlPath, "chain-id"); err == nil {
		status.ChainID = chainID
	}

	// Read evm-chain-id from app.toml
	appTomlPath := filepath.Join(home, "config", "app.toml")
	if evmChainIDStr, err := readTomlValue(appTomlPath, "evm-chain-id"); err == nil {
		status.EVMChainID, _ = strconv.ParseUint(evmChainIDStr, 10, 64)
	}

	// Get peer count from net_info
	resp, err = client.Get("http://localhost:26657/net_info")
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var netInfo struct {
			Result struct {
				NPeers string `json:"n_peers"`
			} `json:"result"`
		}
		if json.Unmarshal(body, &netInfo) == nil {
			status.PeerCount, _ = strconv.Atoi(netInfo.Result.NPeers)
		}
	}

	return status, nil
}

// GetLocalCapabilities detects local node capabilities.
func GetLocalCapabilities(home string) (*MonitorCapabilities, error) {
	caps := &MonitorCapabilities{
		Ports: PortConfig{
			P2P:    26656,
			RPC:    26657,
			REST:   1317,
			GRPC:   9090,
			EVMRPC: 8545,
		},
	}

	// Read pruning from app.toml
	appTomlPath := filepath.Join(home, "config", "app.toml")
	if pruning, err := readTomlValue(appTomlPath, "pruning"); err == nil {
		caps.PruningMode = pruning
	}

	// Check for state-sync (snapshot-interval > 0)
	if interval, err := readTomlValue(appTomlPath, "snapshot-interval"); err == nil {
		if i, _ := strconv.Atoi(interval); i > 0 {
			caps.SnapshotsEnabled = true
		}
	}

	// Detect services by checking local ports
	caps.Services = detectServices()

	return caps, nil
}

// detectServices checks which services are listening locally.
func detectServices() ServiceFlags {
	services := ServiceFlags{}

	// Check each port
	if isPortOpen(26657) {
		services.RPC = true
	}
	if isPortOpen(1317) {
		services.REST = true
	}
	if isPortOpen(9090) {
		services.GRPC = true
	}
	if isPortOpen(8545) {
		services.EVMRPC = true
	}

	return services
}

// isPortOpen checks if a port is listening on localhost.
func isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// readTomlValue reads a simple key=value from a TOML file.
func readTomlValue(path, key string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+" =") || strings.HasPrefix(line, key+"=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				value = strings.Trim(value, "\"")
				return value, nil
			}
		}
	}

	return "", fmt.Errorf("key %s not found", key)
}

// SignHeartbeat creates a signed heartbeat payload.
func SignHeartbeat(keys *MonitorKeys, network string, status *MonitorStatus, caps *MonitorCapabilities) (*HeartbeatPayload, error) {
	timestamp := time.Now().Unix()

	// Generate nonce
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	// Create message to sign: node_id|network|timestamp_unix|nonce
	message := fmt.Sprintf("%s|%s|%d|%s", keys.NodeID, network, timestamp, nonce)

	// Sign with ed25519
	signature := ed25519.Sign(keys.PrivateKey, []byte(message))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	return &HeartbeatPayload{
		NodeID:        keys.NodeID,
		Network:       network,
		TimestampUnix: timestamp,
		Nonce:         nonce,
		Signature:     signatureB64,
		Status: HeartbeatStatus{
			Height:             status.Height,
			CatchingUp:         status.CatchingUp,
			EarliestBlockHeight: status.EarliestBlockHeight,
			VersionMonod:       status.VersionMonod,
			VersionMonoctl:     status.VersionMonoctl,
			ChainID:            status.ChainID,
			EVMChainID:         status.EVMChainID,
			PeerCount:          status.PeerCount,
		},
		Capabilities: *caps,
	}, nil
}

// SendHeartbeat sends a heartbeat to the node-monitor API.
func SendHeartbeat(ctx context.Context, apiEndpoint string, payload *HeartbeatPayload) (*HeartbeatResponse, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint+"/v1/heartbeat", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send heartbeat: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("heartbeat failed: %s (HTTP %d)", string(body), resp.StatusCode)
	}

	var result HeartbeatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// StartRegistration begins the registration process.
func StartRegistration(ctx context.Context, apiEndpoint string, keys *MonitorKeys, network, moniker, role string) (*RegistrationStartResponse, error) {
	req := RegistrationStartRequest{
		NodeID:    keys.NodeID,
		Network:   network,
		Moniker:   moniker,
		Role:      role,
		PublicKey: base64.StdEncoding.EncodeToString(keys.PublicKey),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint+"/v1/register/start", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registration failed: %s (HTTP %d)", string(body), resp.StatusCode)
	}

	var result RegistrationStartResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// VisibilityResponse is the response from setting visibility.
type VisibilityResponse struct {
	Success    bool   `json:"success"`
	NodeID     string `json:"node_id"`
	Visibility string `json:"visibility"`
}

// SetVisibility changes the public visibility of a node.
func SetVisibility(ctx context.Context, apiEndpoint string, keys *MonitorKeys, network, visibility string) (*VisibilityResponse, error) {
	timestamp := time.Now().Unix()

	// Generate nonce
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	// Create message to sign (same format as heartbeat)
	message := fmt.Sprintf("%s|%s|%d|%s", keys.NodeID, network, timestamp, nonce)
	signature := ed25519.Sign(keys.PrivateKey, []byte(message))
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	req := struct {
		NodeID        string `json:"node_id"`
		Network       string `json:"network"`
		Visibility    string `json:"visibility"`
		TimestampUnix int64  `json:"timestamp_unix"`
		Nonce         string `json:"nonce"`
		Signature     string `json:"signature"`
	}{
		NodeID:        keys.NodeID,
		Network:       network,
		Visibility:    visibility,
		TimestampUnix: timestamp,
		Nonce:         nonce,
		Signature:     signatureB64,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint+"/v1/visibility", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("visibility change failed: %s (HTTP %d)", string(body), resp.StatusCode)
	}

	var result VisibilityResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// GenerateSystemdTimer generates a systemd timer unit for heartbeats.
func GenerateSystemdTimer(network, home, user string) (service, timer string) {
	// Using template-based service (the @.service pattern)
	_ = strings.ToLower(network) // Network is passed via %i

	service = fmt.Sprintf(`[Unit]
Description=Monolythium Node Monitor Heartbeat (%%i)
After=network-online.target

[Service]
Type=oneshot
User=%s
ExecStart=/usr/local/bin/monoctl monitor heartbeat --network %%i --home %s
`, user, home)

	timer = fmt.Sprintf(`[Unit]
Description=Monolythium Node Monitor Heartbeat Timer (%%i)

[Timer]
OnBootSec=30
OnUnitActiveSec=30s
AccuracySec=5s

[Install]
WantedBy=timers.target
`)

	return service, timer
}

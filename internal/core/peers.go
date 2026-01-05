package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Peer represents a network peer entry.
type Peer struct {
	NodeID  string `json:"node_id"`
	Address string `json:"address"`
	Port    int    `json:"port,omitempty"`
}

// peersRegistryRaw is used for initial JSON parsing with flexible peer formats.
type peersRegistryRaw struct {
	NetworkName          string            `json:"network_name"`
	ChainID              string            `json:"chain_id"`
	EVMChainID           uint64            `json:"evm_chain_id"`
	GenesisSHA           string            `json:"genesis_sha256"`
	GenesisURL           string            `json:"genesis_url"`
	Seeds                []json.RawMessage `json:"seeds"`
	Peers                []json.RawMessage `json:"peers"`
	PersistentPeers      []json.RawMessage `json:"persistent_peers"`
	BootstrapPeers       []json.RawMessage `json:"bootstrap_peers"`
	TrustedRPCEndpoints  []string          `json:"trusted_rpc_endpoints"`
	PortScheme           *PortScheme       `json:"port_scheme"`
	RPCEndpoints         *RPCEndpoints     `json:"rpc_endpoints"`
}

// PortScheme defines the port configuration for different node types.
type PortScheme struct {
	Seeds      *PortPair            `json:"seeds"`
	Validators map[string]*PortPair `json:"validators"`
}

// PortPair holds P2P and RPC port numbers.
type PortPair struct {
	P2P int `json:"p2p"`
	RPC int `json:"rpc"`
}

// RPCEndpoints holds public RPC endpoint URLs.
type RPCEndpoints struct {
	CometRPC   string `json:"comet_rpc"`
	CosmosREST string `json:"cosmos_rest"`
	EVMRPC     string `json:"evm_rpc"`
}

// PeersRegistry represents a peers.json file.
type PeersRegistry struct {
	NetworkName          string
	ChainID              string
	EVMChainID           uint64
	GenesisSHA           string
	GenesisURL           string
	Seeds                []Peer
	Peers                []Peer
	PersistentPeers      []Peer
	BootstrapPeers       []Peer   // Archive nodes for deterministic genesis sync (pex=false)
	TrustedRPCEndpoints  []string // Trusted RPC endpoints for state sync verification
	PortScheme           *PortScheme
	RPCEndpoints         *RPCEndpoints
}

// nodeIDRegex validates a Tendermint node ID (40 hex chars).
var nodeIDRegex = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)

// peerStringRegex matches "nodeid@host:port" format.
var peerStringRegex = regexp.MustCompile(`^([a-fA-F0-9]{40})@([a-zA-Z0-9.-]+):(\d+)$`)

// ValidatePeer validates a peer entry.
func ValidatePeer(p Peer) error {
	if p.NodeID == "" {
		return fmt.Errorf("peer missing node_id")
	}
	if !nodeIDRegex.MatchString(p.NodeID) {
		return fmt.Errorf("invalid node_id format: %s (expected 40 hex chars)", p.NodeID)
	}
	if p.Address == "" {
		return fmt.Errorf("peer missing address")
	}
	return nil
}

// parsePeerString parses a peer string in format "nodeid@host:port".
func parsePeerString(s string) (Peer, error) {
	matches := peerStringRegex.FindStringSubmatch(s)
	if matches == nil {
		return Peer{}, fmt.Errorf("invalid peer string format: %s (expected nodeid@host:port)", s)
	}

	port, err := strconv.Atoi(matches[3])
	if err != nil {
		return Peer{}, fmt.Errorf("invalid port in peer string: %s", matches[3])
	}

	return Peer{
		NodeID:  strings.ToLower(matches[1]),
		Address: matches[2],
		Port:    port,
	}, nil
}

// parsePeerElement parses a peer from either string or object format.
func parsePeerElement(raw json.RawMessage) (Peer, error) {
	// Try string format first (canonical in mono-core-peers)
	var peerStr string
	if err := json.Unmarshal(raw, &peerStr); err == nil {
		return parsePeerString(peerStr)
	}

	// Try object format (legacy)
	var peerObj Peer
	if err := json.Unmarshal(raw, &peerObj); err == nil {
		return peerObj, nil
	}

	return Peer{}, fmt.Errorf("peer must be string \"nodeid@host:port\" or object {node_id, address, port}")
}

// parsePeerArray parses an array of peers in either format.
func parsePeerArray(raw []json.RawMessage, fieldName string) ([]Peer, error) {
	if raw == nil {
		return nil, nil
	}

	peers := make([]Peer, 0, len(raw))
	for i, elem := range raw {
		peer, err := parsePeerElement(elem)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", fieldName, i, err)
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

// ParsePeersRegistry parses and validates a peers.json file.
// Supports both string format ("nodeid@host:port") and object format.
func ParsePeersRegistry(data []byte) (*PeersRegistry, error) {
	var raw peersRegistryRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse peers.json: %w", err)
	}

	if raw.ChainID == "" {
		return nil, fmt.Errorf("peers.json missing chain_id")
	}

	seeds, err := parsePeerArray(raw.Seeds, "seeds")
	if err != nil {
		return nil, err
	}

	peers, err := parsePeerArray(raw.Peers, "peers")
	if err != nil {
		return nil, err
	}

	persistentPeers, err := parsePeerArray(raw.PersistentPeers, "persistent_peers")
	if err != nil {
		return nil, err
	}

	bootstrapPeers, err := parsePeerArray(raw.BootstrapPeers, "bootstrap_peers")
	if err != nil {
		return nil, err
	}

	// Validate all parsed peers
	for i, p := range seeds {
		if err := ValidatePeer(p); err != nil {
			return nil, fmt.Errorf("seeds[%d]: %w", i, err)
		}
	}

	for i, p := range peers {
		if err := ValidatePeer(p); err != nil {
			return nil, fmt.Errorf("peers[%d]: %w", i, err)
		}
	}

	for i, p := range persistentPeers {
		if err := ValidatePeer(p); err != nil {
			return nil, fmt.Errorf("persistent_peers[%d]: %w", i, err)
		}
	}

	for i, p := range bootstrapPeers {
		if err := ValidatePeer(p); err != nil {
			return nil, fmt.Errorf("bootstrap_peers[%d]: %w", i, err)
		}
	}

	return &PeersRegistry{
		NetworkName:          raw.NetworkName,
		ChainID:              raw.ChainID,
		EVMChainID:           raw.EVMChainID,
		GenesisSHA:           raw.GenesisSHA,
		GenesisURL:           raw.GenesisURL,
		Seeds:                seeds,
		Peers:                peers,
		PersistentPeers:      persistentPeers,
		BootstrapPeers:       bootstrapPeers,
		TrustedRPCEndpoints:  raw.TrustedRPCEndpoints,
		PortScheme:           raw.PortScheme,
		RPCEndpoints:         raw.RPCEndpoints,
	}, nil
}

// GetSeedP2PPort returns the P2P port for seed nodes, with a default of 26656.
func (r *PeersRegistry) GetSeedP2PPort() int {
	if r.PortScheme != nil && r.PortScheme.Seeds != nil && r.PortScheme.Seeds.P2P != 0 {
		return r.PortScheme.Seeds.P2P
	}
	return 26656
}

// GetValidatorP2PPort returns the P2P port for a named validator, with a default of 26656.
func (r *PeersRegistry) GetValidatorP2PPort(name string) int {
	if r.PortScheme != nil && r.PortScheme.Validators != nil {
		if pp, ok := r.PortScheme.Validators[name]; ok && pp.P2P != 0 {
			return pp.P2P
		}
		// Try "default" fallback
		if pp, ok := r.PortScheme.Validators["default"]; ok && pp.P2P != 0 {
			return pp.P2P
		}
	}
	return 26656
}

// ValidatePeersRegistry validates that the peers registry matches the expected genesis.
func ValidatePeersRegistry(reg *PeersRegistry, expectedChainID, expectedGenesisSHA string) error {
	if reg.ChainID != expectedChainID {
		return fmt.Errorf("chain_id mismatch: expected %s, got %s", expectedChainID, reg.ChainID)
	}

	if expectedGenesisSHA != "" && reg.GenesisSHA != expectedGenesisSHA {
		return fmt.Errorf("genesis_sha256 mismatch: expected %s, got %s", expectedGenesisSHA, reg.GenesisSHA)
	}

	return nil
}

// PeerString formats a peer as node_id@address:port.
func (p Peer) String() string {
	port := p.Port
	if port == 0 {
		port = 26656
	}
	return fmt.Sprintf("%s@%s:%d", p.NodeID, p.Address, port)
}

// PeersToString converts a list of peers to a comma-separated string.
func PeersToString(peers []Peer) string {
	if len(peers) == 0 {
		return ""
	}
	strs := make([]string, len(peers))
	for i, p := range peers {
		strs[i] = p.String()
	}
	return strings.Join(strs, ",")
}

// MergePeers merges two peer lists, deduplicating by node ID.
func MergePeers(a, b []Peer) []Peer {
	seen := make(map[string]bool)
	result := make([]Peer, 0, len(a)+len(b))

	for _, p := range a {
		if !seen[p.NodeID] {
			seen[p.NodeID] = true
			result = append(result, p)
		}
	}

	for _, p := range b {
		if !seen[p.NodeID] {
			seen[p.NodeID] = true
			result = append(result, p)
		}
	}

	return result
}

package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Peer represents a network peer entry.
type Peer struct {
	NodeID  string `json:"node_id"`
	Address string `json:"address"`
	Port    int    `json:"port,omitempty"`
}

// PeersRegistry represents a peers.json file.
type PeersRegistry struct {
	ChainID      string `json:"chain_id"`
	GenesisSHA   string `json:"genesis_sha256"`
	Peers        []Peer `json:"peers"`
	PersistentPeers []Peer `json:"persistent_peers,omitempty"`
}

// nodeIDRegex validates a Tendermint node ID (40 hex chars).
var nodeIDRegex = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)

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

// ParsePeersRegistry parses and validates a peers.json file.
func ParsePeersRegistry(data []byte) (*PeersRegistry, error) {
	var reg PeersRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to parse peers.json: %w", err)
	}

	if reg.ChainID == "" {
		return nil, fmt.Errorf("peers.json missing chain_id")
	}

	for i, p := range reg.Peers {
		if err := ValidatePeer(p); err != nil {
			return nil, fmt.Errorf("peers[%d]: %w", i, err)
		}
	}

	for i, p := range reg.PersistentPeers {
		if err := ValidatePeer(p); err != nil {
			return nil, fmt.Errorf("persistent_peers[%d]: %w", i, err)
		}
	}

	return &reg, nil
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

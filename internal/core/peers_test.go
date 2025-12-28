package core

import (
	"testing"
)

func TestValidatePeer(t *testing.T) {
	tests := []struct {
		name    string
		peer    Peer
		wantErr bool
	}{
		{
			name: "valid peer",
			peer: Peer{
				NodeID:  "abcdef1234567890abcdef1234567890abcdef12",
				Address: "192.168.1.1",
				Port:    26656,
			},
			wantErr: false,
		},
		{
			name: "valid peer without port",
			peer: Peer{
				NodeID:  "abcdef1234567890abcdef1234567890abcdef12",
				Address: "seed.example.com",
			},
			wantErr: false,
		},
		{
			name: "missing node_id",
			peer: Peer{
				Address: "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "invalid node_id format",
			peer: Peer{
				NodeID:  "invalid",
				Address: "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "missing address",
			peer: Peer{
				NodeID: "abcdef1234567890abcdef1234567890abcdef12",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePeer(tt.peer)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePeer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParsePeersRegistry(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "valid registry",
			data: `{
				"chain_id": "mono-sprint-1",
				"genesis_sha256": "abc123",
				"peers": [
					{"node_id": "abcdef1234567890abcdef1234567890abcdef12", "address": "192.168.1.1", "port": 26656}
				]
			}`,
			wantErr: false,
		},
		{
			name: "missing chain_id",
			data: `{
				"peers": []
			}`,
			wantErr: true,
		},
		{
			name: "invalid peer",
			data: `{
				"chain_id": "mono-sprint-1",
				"peers": [
					{"node_id": "invalid", "address": "192.168.1.1"}
				]
			}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePeersRegistry([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePeersRegistry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePeersRegistry(t *testing.T) {
	reg := &PeersRegistry{
		ChainID:    "mono-sprint-1",
		GenesisSHA: "abc123",
	}

	// Matching chain ID
	err := ValidatePeersRegistry(reg, "mono-sprint-1", "")
	if err != nil {
		t.Errorf("ValidatePeersRegistry() unexpected error: %v", err)
	}

	// Mismatched chain ID
	err = ValidatePeersRegistry(reg, "mono-test-1", "")
	if err == nil {
		t.Error("ValidatePeersRegistry() expected error for chain_id mismatch")
	}

	// Matching genesis SHA
	err = ValidatePeersRegistry(reg, "mono-sprint-1", "abc123")
	if err != nil {
		t.Errorf("ValidatePeersRegistry() unexpected error: %v", err)
	}

	// Mismatched genesis SHA
	err = ValidatePeersRegistry(reg, "mono-sprint-1", "xyz789")
	if err == nil {
		t.Error("ValidatePeersRegistry() expected error for genesis_sha mismatch")
	}
}

func TestPeerString(t *testing.T) {
	tests := []struct {
		peer Peer
		want string
	}{
		{
			peer: Peer{
				NodeID:  "abcdef1234567890abcdef1234567890abcdef12",
				Address: "192.168.1.1",
				Port:    26656,
			},
			want: "abcdef1234567890abcdef1234567890abcdef12@192.168.1.1:26656",
		},
		{
			peer: Peer{
				NodeID:  "abcdef1234567890abcdef1234567890abcdef12",
				Address: "seed.example.com",
				Port:    0, // should default to 26656
			},
			want: "abcdef1234567890abcdef1234567890abcdef12@seed.example.com:26656",
		},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.peer.String(); got != tt.want {
				t.Errorf("Peer.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPeersToString(t *testing.T) {
	peers := []Peer{
		{NodeID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Address: "host1.com", Port: 26656},
		{NodeID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Address: "host2.com", Port: 26656},
	}

	got := PeersToString(peers)
	want := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa@host1.com:26656,bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb@host2.com:26656"
	if got != want {
		t.Errorf("PeersToString() = %v, want %v", got, want)
	}

	// Empty list
	got = PeersToString(nil)
	if got != "" {
		t.Errorf("PeersToString(nil) = %v, want empty", got)
	}
}

func TestMergePeers(t *testing.T) {
	a := []Peer{
		{NodeID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Address: "host1.com"},
		{NodeID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Address: "host2.com"},
	}
	b := []Peer{
		{NodeID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Address: "host2-updated.com"}, // duplicate
		{NodeID: "cccccccccccccccccccccccccccccccccccccccc", Address: "host3.com"},
	}

	merged := MergePeers(a, b)

	if len(merged) != 3 {
		t.Errorf("MergePeers() returned %d peers, want 3", len(merged))
	}

	// Verify first occurrence wins (host2.com, not host2-updated.com)
	for _, p := range merged {
		if p.NodeID == "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
			if p.Address != "host2.com" {
				t.Errorf("MergePeers() should keep first occurrence, got %v", p.Address)
			}
		}
	}
}

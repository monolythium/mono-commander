package core

import (
	"testing"
)

func TestGetNetwork(t *testing.T) {
	tests := []struct {
		name      string
		network   NetworkName
		wantChain string
		wantEVM   uint64
		wantErr   bool
	}{
		{
			name:      "Localnet",
			network:   NetworkLocalnet,
			wantChain: "mono-local-1",
			wantEVM:   262145,
			wantErr:   false,
		},
		{
			name:      "Sprintnet",
			network:   NetworkSprintnet,
			wantChain: "mono-sprint-1",
			wantEVM:   262146,
			wantErr:   false,
		},
		{
			name:      "Testnet",
			network:   NetworkTestnet,
			wantChain: "mono-test-1",
			wantEVM:   262147,
			wantErr:   false,
		},
		{
			name:      "Mainnet",
			network:   NetworkMainnet,
			wantChain: "mono-1",
			wantEVM:   262148,
			wantErr:   false,
		},
		{
			name:    "Unknown",
			network: NetworkName("unknown"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNetwork(tt.network)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.ChainID != tt.wantChain {
					t.Errorf("GetNetwork() ChainID = %v, want %v", got.ChainID, tt.wantChain)
				}
				if got.EVMChainID != tt.wantEVM {
					t.Errorf("GetNetwork() EVMChainID = %v, want %v", got.EVMChainID, tt.wantEVM)
				}
			}
		})
	}
}

func TestGetNetworkByChainID(t *testing.T) {
	tests := []struct {
		name     string
		chainID  string
		wantName NetworkName
		wantErr  bool
	}{
		{"mono-local-1", "mono-local-1", NetworkLocalnet, false},
		{"mono-sprint-1", "mono-sprint-1", NetworkSprintnet, false},
		{"mono-test-1", "mono-test-1", NetworkTestnet, false},
		{"mono-1", "mono-1", NetworkMainnet, false},
		{"unknown", "unknown-chain", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNetworkByChainID(tt.chainID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNetworkByChainID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Name != tt.wantName {
				t.Errorf("GetNetworkByChainID() Name = %v, want %v", got.Name, tt.wantName)
			}
		})
	}
}

func TestParseNetworkName(t *testing.T) {
	tests := []struct {
		input   string
		want    NetworkName
		wantErr bool
	}{
		{"localnet", NetworkLocalnet, false},
		{"Localnet", NetworkLocalnet, false},
		{"LOCALNET", NetworkLocalnet, false},
		{"sprintnet", NetworkSprintnet, false},
		{"testnet", NetworkTestnet, false},
		{"mainnet", NetworkMainnet, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseNetworkName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNetworkName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseNetworkName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestListNetworks(t *testing.T) {
	networks := ListNetworks()
	if len(networks) != 4 {
		t.Errorf("ListNetworks() returned %d networks, want 4", len(networks))
	}

	// Verify order
	expected := []NetworkName{NetworkLocalnet, NetworkSprintnet, NetworkTestnet, NetworkMainnet}
	for i, n := range networks {
		if n.Name != expected[i] {
			t.Errorf("ListNetworks()[%d] = %v, want %v", i, n.Name, expected[i])
		}
	}
}

func TestEVMChainIDHex(t *testing.T) {
	tests := []struct {
		network NetworkName
		want    string
	}{
		{NetworkLocalnet, "0x40001"},
		{NetworkSprintnet, "0x40002"},
		{NetworkTestnet, "0x40003"},
		{NetworkMainnet, "0x40004"},
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			n, _ := GetNetwork(tt.network)
			if got := n.EVMChainIDHex(); got != tt.want {
				t.Errorf("EVMChainIDHex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeedString(t *testing.T) {
	// Sprintnet should have seeds
	n, _ := GetNetwork(NetworkSprintnet)
	seeds := n.SeedString(26656)
	if seeds == "" {
		t.Error("SeedString() returned empty for Sprintnet")
	}

	// Localnet should have no seeds
	n, _ = GetNetwork(NetworkLocalnet)
	seeds = n.SeedString(26656)
	if seeds != "" {
		t.Errorf("SeedString() = %q for Localnet, want empty", seeds)
	}
}

func TestDefaultPeersURLSelection(t *testing.T) {
	// Verify that each public network has a default PeersURL pointing to mono-core-peers
	tests := []struct {
		network     NetworkName
		wantBaseURL string
		wantEmpty   bool
	}{
		{
			network:     NetworkSprintnet,
			wantBaseURL: "https://raw.githubusercontent.com/monolythium/mono-core-peers/prod/networks/sprintnet/peers.json",
			wantEmpty:   false,
		},
		{
			network:     NetworkTestnet,
			wantBaseURL: "https://raw.githubusercontent.com/monolythium/mono-core-peers/prod/networks/testnet/peers.json",
			wantEmpty:   false,
		},
		{
			network:     NetworkMainnet,
			wantBaseURL: "https://raw.githubusercontent.com/monolythium/mono-core-peers/prod/networks/mainnet/peers.json",
			wantEmpty:   false,
		},
		{
			network:   NetworkLocalnet,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			n, err := GetNetwork(tt.network)
			if err != nil {
				t.Fatalf("GetNetwork() error = %v", err)
			}

			if tt.wantEmpty {
				if n.PeersURL != "" {
					t.Errorf("PeersURL = %q, want empty for %s", n.PeersURL, tt.network)
				}
			} else {
				if n.PeersURL != tt.wantBaseURL {
					t.Errorf("PeersURL = %q, want %q", n.PeersURL, tt.wantBaseURL)
				}
			}
		})
	}
}

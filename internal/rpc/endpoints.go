package rpc

import (
	"fmt"
)

// NetworkName constants (duplicated to avoid import cycle with core).
const (
	networkLocalnet  = "Localnet"
	networkSprintnet = "Sprintnet"
	networkTestnet   = "Testnet"
	networkMainnet   = "Mainnet"
)

// Endpoints holds RPC endpoints for a network.
type Endpoints struct {
	CometRPC   string // e.g., http://localhost:26657
	CosmosREST string // e.g., http://localhost:1317
	EVMRPC     string // e.g., http://localhost:8545
}

// DefaultPorts defines default ports for each RPC type.
type DefaultPorts struct {
	CometRPC   int
	CosmosREST int
	EVMRPC     int
}

// GetDefaultPorts returns default ports for a network.
func GetDefaultPorts(network string) DefaultPorts {
	// Localnet uses standard ports
	// Other networks may use remote endpoints
	switch network {
	case networkLocalnet:
		return DefaultPorts{
			CometRPC:   26657,
			CosmosREST: 1317,
			EVMRPC:     8545,
		}
	default:
		// For remote networks, assume local node with standard ports
		return DefaultPorts{
			CometRPC:   26657,
			CosmosREST: 1317,
			EVMRPC:     8545,
		}
	}
}

// GetLocalEndpoints returns endpoints for a local node.
func GetLocalEndpoints(host string, ports DefaultPorts) Endpoints {
	if host == "" {
		host = "localhost"
	}
	return Endpoints{
		CometRPC:   fmt.Sprintf("http://%s:%d", host, ports.CometRPC),
		CosmosREST: fmt.Sprintf("http://%s:%d", host, ports.CosmosREST),
		EVMRPC:     fmt.Sprintf("http://%s:%d", host, ports.EVMRPC),
	}
}

// GetRemoteEndpoints returns endpoints for remote network infrastructure.
// These are placeholder URLs; real endpoints would be configured per network.
func GetRemoteEndpoints(network string) Endpoints {
	switch network {
	case networkSprintnet:
		return Endpoints{
			CometRPC:   "https://rpc.sprintnet.monolythium.com",
			CosmosREST: "https://api.sprintnet.monolythium.com",
			EVMRPC:     "https://evm.sprintnet.monolythium.com",
		}
	case networkTestnet:
		return Endpoints{
			CometRPC:   "https://rpc.testnet.monolythium.com",
			CosmosREST: "https://api.testnet.monolythium.com",
			EVMRPC:     "https://evm.testnet.monolythium.com",
		}
	case networkMainnet:
		return Endpoints{
			CometRPC:   "https://rpc.monolythium.com",
			CosmosREST: "https://api.monolythium.com",
			EVMRPC:     "https://evm.monolythium.com",
		}
	default:
		// Localnet uses local endpoints
		ports := GetDefaultPorts(network)
		return GetLocalEndpoints("localhost", ports)
	}
}

// EndpointOptions allows overriding default endpoints.
type EndpointOptions struct {
	CometRPC   string
	CosmosREST string
	EVMRPC     string
	Host       string
	UseRemote  bool
}

// ResolveEndpoints resolves endpoints with optional overrides.
// network should be a string representation of the network name (e.g., "Localnet", "Sprintnet").
func ResolveEndpoints(network string, opts EndpointOptions) Endpoints {
	var endpoints Endpoints

	if opts.UseRemote {
		endpoints = GetRemoteEndpoints(network)
	} else {
		ports := GetDefaultPorts(network)
		host := opts.Host
		if host == "" {
			host = "localhost"
		}
		endpoints = GetLocalEndpoints(host, ports)
	}

	// Apply overrides
	if opts.CometRPC != "" {
		endpoints.CometRPC = opts.CometRPC
	}
	if opts.CosmosREST != "" {
		endpoints.CosmosREST = opts.CosmosREST
	}
	if opts.EVMRPC != "" {
		endpoints.EVMRPC = opts.EVMRPC
	}

	return endpoints
}

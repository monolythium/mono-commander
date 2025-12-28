package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetNodeStatus(t *testing.T) {
	// Create mock Comet RPC server
	cometServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			resp := `{
				"jsonrpc": "2.0",
				"id": -1,
				"result": {
					"node_info": {
						"network": "mono-local-1",
						"moniker": "test-node",
						"version": "0.38.0"
					},
					"sync_info": {
						"latest_block_height": "12345",
						"catching_up": false
					}
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(resp))
		case "/net_info":
			resp := `{
				"jsonrpc": "2.0",
				"id": -1,
				"result": {
					"n_peers": "5"
				}
			}`
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(resp))
		default:
			http.NotFound(w, r)
		}
	}))
	defer cometServer.Close()

	opts := StatusOptions{
		Network: NetworkLocalnet,
		Endpoints: Endpoints{
			CometRPC:   cometServer.URL,
			CosmosREST: "http://localhost:1317",
			EVMRPC:     "http://localhost:8545",
		},
	}

	status, err := GetNodeStatus(opts)
	if err != nil {
		t.Fatalf("GetNodeStatus() error = %v", err)
	}

	if status.ChainID != "mono-local-1" {
		t.Errorf("ChainID = %q, want %q", status.ChainID, "mono-local-1")
	}
	if status.Moniker != "test-node" {
		t.Errorf("Moniker = %q, want %q", status.Moniker, "test-node")
	}
	if status.LatestHeight != 12345 {
		t.Errorf("LatestHeight = %d, want %d", status.LatestHeight, 12345)
	}
	if status.CatchingUp != false {
		t.Errorf("CatchingUp = %v, want %v", status.CatchingUp, false)
	}
	if status.PeersCount != 5 {
		t.Errorf("PeersCount = %d, want %d", status.PeersCount, 5)
	}
}

func TestGetNodeStatus_ConnectionError(t *testing.T) {
	opts := StatusOptions{
		Network: NetworkLocalnet,
		Endpoints: Endpoints{
			CometRPC:   "http://localhost:99999",
			CosmosREST: "http://localhost:99998",
			EVMRPC:     "http://localhost:99997",
		},
	}

	_, err := GetNodeStatus(opts)
	if err == nil {
		t.Error("GetNodeStatus() expected error for invalid endpoint")
	}
}

func TestCheckRPC_AllPass(t *testing.T) {
	// Create mock Comet RPC server
	cometServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"jsonrpc": "2.0",
			"id": -1,
			"result": {
				"node_info": {
					"network": "mono-local-1",
					"moniker": "test-node",
					"version": "0.38.0"
				},
				"sync_info": {
					"latest_block_height": "12345",
					"catching_up": false
				}
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer cometServer.Close()

	// Create mock Cosmos REST server
	cosmosServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"default_node_info": {
				"network": "mono-local-1"
			},
			"application_version": {
				"app_name": "monod"
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer cosmosServer.Close()

	// Create mock EVM RPC server
	evmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		var result string
		switch req.Method {
		case "eth_chainId":
			result = `"0x40001"` // 262145
		case "eth_blockNumber":
			result = `"0x1234"` // 4660
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  json.RawMessage(result),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer evmServer.Close()

	endpoints := Endpoints{
		CometRPC:   cometServer.URL,
		CosmosREST: cosmosServer.URL,
		EVMRPC:     evmServer.URL,
	}

	results := CheckRPC(NetworkLocalnet, endpoints)

	if !results.AllPass {
		t.Errorf("AllPass = %v, want %v", results.AllPass, true)
		for _, r := range results.Results {
			if r.Status == "FAIL" {
				t.Logf("Failed: %s - %s", r.Type, r.Message)
			}
		}
	}

	if len(results.Results) != 3 {
		t.Errorf("len(Results) = %d, want 3", len(results.Results))
	}

	for _, r := range results.Results {
		if r.Status != "PASS" {
			t.Errorf("%s Status = %q, want PASS", r.Type, r.Status)
		}
	}
}

func TestCheckRPC_CometFail(t *testing.T) {
	// Failed Comet server
	cometServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer cometServer.Close()

	// Working Cosmos REST server
	cosmosServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"default_node_info": {"network": "mono-local-1"},
			"application_version": {"app_name": "monod"}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer cosmosServer.Close()

	// Working EVM server
	evmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		var result string
		switch req.Method {
		case "eth_chainId":
			result = `"0x40001"`
		case "eth_blockNumber":
			result = `"0x1234"`
		}
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  json.RawMessage(result),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer evmServer.Close()

	endpoints := Endpoints{
		CometRPC:   cometServer.URL,
		CosmosREST: cosmosServer.URL,
		EVMRPC:     evmServer.URL,
	}

	results := CheckRPC(NetworkLocalnet, endpoints)

	if results.AllPass {
		t.Error("AllPass should be false when Comet fails")
	}

	// Comet should fail
	for _, r := range results.Results {
		if r.Type == "Comet RPC" && r.Status != "FAIL" {
			t.Errorf("Comet RPC Status = %q, want FAIL", r.Status)
		}
	}
}

func TestCheckRPC_EVMChainIDMismatch(t *testing.T) {
	// Working Comet server
	cometServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"jsonrpc": "2.0",
			"id": -1,
			"result": {
				"node_info": {"network": "mono-local-1", "moniker": "test", "version": "0.38.0"},
				"sync_info": {"latest_block_height": "100", "catching_up": false}
			}
		}`
		w.Write([]byte(resp))
	}))
	defer cometServer.Close()

	// Working Cosmos REST server
	cosmosServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := `{
			"default_node_info": {"network": "mono-local-1"},
			"application_version": {"app_name": "monod"}
		}`
		w.Write([]byte(resp))
	}))
	defer cosmosServer.Close()

	// EVM server with wrong chain ID
	evmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		var result string
		switch req.Method {
		case "eth_chainId":
			result = `"0x1"` // Wrong chain ID (1 instead of 262145)
		case "eth_blockNumber":
			result = `"0x1234"`
		}
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  json.RawMessage(result),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer evmServer.Close()

	endpoints := Endpoints{
		CometRPC:   cometServer.URL,
		CosmosREST: cosmosServer.URL,
		EVMRPC:     evmServer.URL,
	}

	results := CheckRPC(NetworkLocalnet, endpoints)

	if results.AllPass {
		t.Error("AllPass should be false when EVM chain ID mismatches")
	}

	// EVM should fail with chain ID mismatch
	for _, r := range results.Results {
		if r.Type == "EVM JSON-RPC" {
			if r.Status != "FAIL" {
				t.Errorf("EVM JSON-RPC Status = %q, want FAIL", r.Status)
			}
			if r.Message == "" {
				t.Error("EVM JSON-RPC should have error message about chain ID mismatch")
			}
		}
	}
}

func TestEndpoints(t *testing.T) {
	endpoints := Endpoints{
		CometRPC:   "http://localhost:26657",
		CosmosREST: "http://localhost:1317",
		EVMRPC:     "http://localhost:8545",
	}

	if endpoints.CometRPC != "http://localhost:26657" {
		t.Errorf("CometRPC = %q, want %q", endpoints.CometRPC, "http://localhost:26657")
	}
	if endpoints.CosmosREST != "http://localhost:1317" {
		t.Errorf("CosmosREST = %q, want %q", endpoints.CosmosREST, "http://localhost:1317")
	}
	if endpoints.EVMRPC != "http://localhost:8545" {
		t.Errorf("EVMRPC = %q, want %q", endpoints.EVMRPC, "http://localhost:8545")
	}
}

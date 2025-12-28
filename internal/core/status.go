package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// NodeStatus represents the status of a Monolythium node.
type NodeStatus struct {
	ChainID       string `json:"chain_id"`
	LatestHeight  int64  `json:"latest_height"`
	CatchingUp    bool   `json:"catching_up"`
	PeersCount    int    `json:"peers_count"`
	Moniker       string `json:"moniker"`
	NodeVersion   string `json:"node_version"`
	ServiceStatus string `json:"service_status,omitempty"`
}

// Endpoints holds RPC endpoints for node operations.
type Endpoints struct {
	CometRPC   string
	CosmosREST string
	EVMRPC     string
}

// StatusOptions holds options for the status command.
type StatusOptions struct {
	Network   NetworkName
	Endpoints Endpoints
}

// GetNodeStatus fetches the status of a node.
func GetNodeStatus(opts StatusOptions) (*NodeStatus, error) {
	client := newHTTPClient()

	// Get node status via Comet RPC
	statusResp, err := fetchCometStatus(client, opts.Endpoints.CometRPC)
	if err != nil {
		return nil, fmt.Errorf("failed to get node status: %w", err)
	}

	height, _ := strconv.ParseInt(statusResp.Result.SyncInfo.LatestBlockHeight, 10, 64)

	status := &NodeStatus{
		ChainID:      statusResp.Result.NodeInfo.Network,
		LatestHeight: height,
		CatchingUp:   statusResp.Result.SyncInfo.CatchingUp,
		Moniker:      statusResp.Result.NodeInfo.Moniker,
		NodeVersion:  statusResp.Result.NodeInfo.Version,
	}

	// Try to get peer count
	netInfo, err := fetchCometNetInfo(client, opts.Endpoints.CometRPC)
	if err == nil {
		peersCount, _ := strconv.Atoi(netInfo.Result.NPeers)
		status.PeersCount = peersCount
	}

	return status, nil
}

// RPCCheckResult represents the result of an RPC check.
type RPCCheckResult struct {
	Endpoint string `json:"endpoint"`
	Type     string `json:"type"`
	Status   string `json:"status"` // "PASS" or "FAIL"
	Message  string `json:"message,omitempty"`
	Details  string `json:"details,omitempty"`
}

// RPCCheckResults holds all RPC check results.
type RPCCheckResults struct {
	Network NetworkName      `json:"network"`
	Results []RPCCheckResult `json:"results"`
	AllPass bool             `json:"all_pass"`
}

// CheckRPC performs RPC health checks on all endpoints.
func CheckRPC(network NetworkName, endpoints Endpoints) *RPCCheckResults {
	results := &RPCCheckResults{
		Network: network,
		Results: make([]RPCCheckResult, 0, 3),
		AllPass: true,
	}

	client := newHTTPClient()

	// Check Comet RPC
	cometResult := checkCometRPC(client, endpoints.CometRPC)
	results.Results = append(results.Results, cometResult)
	if cometResult.Status == "FAIL" {
		results.AllPass = false
	}

	// Check Cosmos REST
	cosmosResult := checkCosmosREST(client, endpoints.CosmosREST)
	results.Results = append(results.Results, cosmosResult)
	if cosmosResult.Status == "FAIL" {
		results.AllPass = false
	}

	// Check EVM RPC
	evmResult := checkEVMRPC(client, endpoints.EVMRPC, network)
	results.Results = append(results.Results, evmResult)
	if evmResult.Status == "FAIL" {
		results.AllPass = false
	}

	return results
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// Comet RPC types
type cometStatusResponse struct {
	Result struct {
		NodeInfo struct {
			Network string `json:"network"`
			Moniker string `json:"moniker"`
			Version string `json:"version"`
		} `json:"node_info"`
		SyncInfo struct {
			LatestBlockHeight string `json:"latest_block_height"`
			CatchingUp        bool   `json:"catching_up"`
		} `json:"sync_info"`
	} `json:"result"`
}

type cometNetInfoResponse struct {
	Result struct {
		NPeers string `json:"n_peers"`
	} `json:"result"`
}

func fetchCometStatus(client *http.Client, baseURL string) (*cometStatusResponse, error) {
	resp, err := client.Get(baseURL + "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, baseURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var status cometStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return &status, nil
}

func fetchCometNetInfo(client *http.Client, baseURL string) (*cometNetInfoResponse, error) {
	resp, err := client.Get(baseURL + "/net_info")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var netInfo cometNetInfoResponse
	if err := json.Unmarshal(body, &netInfo); err != nil {
		return nil, err
	}

	return &netInfo, nil
}

func checkCometRPC(client *http.Client, endpoint string) RPCCheckResult {
	result := RPCCheckResult{
		Endpoint: endpoint,
		Type:     "Comet RPC",
	}

	status, err := fetchCometStatus(client, endpoint)
	if err != nil {
		result.Status = "FAIL"
		result.Message = err.Error()
		return result
	}

	result.Status = "PASS"
	result.Details = fmt.Sprintf("chain=%s height=%s",
		status.Result.NodeInfo.Network,
		status.Result.SyncInfo.LatestBlockHeight)
	return result
}

// Cosmos REST types
type cosmosNodeInfoResponse struct {
	DefaultNodeInfo struct {
		Network string `json:"network"`
	} `json:"default_node_info"`
	ApplicationVersion struct {
		AppName string `json:"app_name"`
	} `json:"application_version"`
}

func checkCosmosREST(client *http.Client, endpoint string) RPCCheckResult {
	result := RPCCheckResult{
		Endpoint: endpoint,
		Type:     "Cosmos REST",
	}

	url := endpoint + "/cosmos/base/tendermint/v1beta1/node_info"
	resp, err := client.Get(url)
	if err != nil {
		result.Status = "FAIL"
		result.Message = err.Error()
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return result
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Status = "FAIL"
		result.Message = err.Error()
		return result
	}

	var info cosmosNodeInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		result.Status = "FAIL"
		result.Message = err.Error()
		return result
	}

	result.Status = "PASS"
	result.Details = fmt.Sprintf("network=%s app=%s",
		info.DefaultNodeInfo.Network,
		info.ApplicationVersion.AppName)
	return result
}

// EVM JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

func evmCall(client *http.Client, endpoint, method string, params []interface{}) (*jsonRPCResponse, error) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

func checkEVMRPC(client *http.Client, endpoint string, network NetworkName) RPCCheckResult {
	result := RPCCheckResult{
		Endpoint: endpoint,
		Type:     "EVM JSON-RPC",
	}

	// Get chain ID
	chainResp, err := evmCall(client, endpoint, "eth_chainId", []interface{}{})
	if err != nil {
		result.Status = "FAIL"
		result.Message = err.Error()
		return result
	}

	var hexChainID string
	if err := json.Unmarshal(chainResp.Result, &hexChainID); err != nil {
		result.Status = "FAIL"
		result.Message = "failed to parse chain ID"
		return result
	}

	chainID, err := strconv.ParseUint(hexChainID, 0, 64)
	if err != nil {
		result.Status = "FAIL"
		result.Message = "failed to parse chain ID hex"
		return result
	}

	// Get block number
	blockResp, err := evmCall(client, endpoint, "eth_blockNumber", []interface{}{})
	if err != nil {
		result.Status = "FAIL"
		result.Message = err.Error()
		return result
	}

	var hexBlockNum string
	if err := json.Unmarshal(blockResp.Result, &hexBlockNum); err != nil {
		result.Status = "FAIL"
		result.Message = "failed to parse block number"
		return result
	}

	blockNum, _ := strconv.ParseUint(hexBlockNum, 0, 64)

	// Verify chain ID matches network
	expectedNetwork, _ := GetNetwork(network)
	if chainID != expectedNetwork.EVMChainID {
		result.Status = "FAIL"
		result.Message = fmt.Sprintf("chain ID mismatch: expected %d, got %d",
			expectedNetwork.EVMChainID, chainID)
		return result
	}

	result.Status = "PASS"
	result.Details = fmt.Sprintf("chainId=%s block=%d", hexChainID, blockNum)
	return result
}

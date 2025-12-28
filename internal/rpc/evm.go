package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// EVMClient is a client for EVM JSON-RPC.
type EVMClient struct {
	BaseURL string
	Client  *http.Client
}

// NewEVMClient creates a new EVM JSON-RPC client.
func NewEVMClient(baseURL string) *EVMClient {
	return &EVMClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// JSONRPCRequest represents a JSON-RPC request.
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// JSONRPCError represents a JSON-RPC error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// call makes a JSON-RPC call.
func (c *EVMClient) call(method string, params []interface{}) (*JSONRPCResponse, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.Client.Post(c.BaseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, c.BaseURL)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

// ChainID fetches the chain ID via eth_chainId.
func (c *EVMClient) ChainID() (uint64, string, error) {
	resp, err := c.call("eth_chainId", []interface{}{})
	if err != nil {
		return 0, "", err
	}

	var hexChainID string
	if err := json.Unmarshal(resp.Result, &hexChainID); err != nil {
		return 0, "", fmt.Errorf("failed to parse chain ID: %w", err)
	}

	chainID, err := strconv.ParseUint(hexChainID, 0, 64)
	if err != nil {
		return 0, hexChainID, fmt.Errorf("failed to parse chain ID hex: %w", err)
	}

	return chainID, hexChainID, nil
}

// BlockNumber fetches the latest block number via eth_blockNumber.
func (c *EVMClient) BlockNumber() (uint64, string, error) {
	resp, err := c.call("eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, "", err
	}

	var hexBlockNumber string
	if err := json.Unmarshal(resp.Result, &hexBlockNumber); err != nil {
		return 0, "", fmt.Errorf("failed to parse block number: %w", err)
	}

	blockNumber, err := strconv.ParseUint(hexBlockNumber, 0, 64)
	if err != nil {
		return 0, hexBlockNumber, fmt.Errorf("failed to parse block number hex: %w", err)
	}

	return blockNumber, hexBlockNumber, nil
}

// ClientVersion fetches the client version via web3_clientVersion.
func (c *EVMClient) ClientVersion() (string, error) {
	resp, err := c.call("web3_clientVersion", []interface{}{})
	if err != nil {
		return "", err
	}

	var version string
	if err := json.Unmarshal(resp.Result, &version); err != nil {
		return "", fmt.Errorf("failed to parse client version: %w", err)
	}

	return version, nil
}

// NetVersion fetches the network version via net_version.
func (c *EVMClient) NetVersion() (string, error) {
	resp, err := c.call("net_version", []interface{}{})
	if err != nil {
		return "", err
	}

	var version string
	if err := json.Unmarshal(resp.Result, &version); err != nil {
		return "", fmt.Errorf("failed to parse net version: %w", err)
	}

	return version, nil
}

// Package rpc provides RPC clients for Comet, Cosmos REST, and EVM JSON-RPC.
package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CometClient is a client for CometBFT RPC.
type CometClient struct {
	BaseURL string
	Client  *http.Client
}

// NewCometClient creates a new CometBFT RPC client.
func NewCometClient(baseURL string) *CometClient {
	return &CometClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// StatusResponse represents the /status RPC response.
type StatusResponse struct {
	Result struct {
		NodeInfo struct {
			Network string `json:"network"`
			Moniker string `json:"moniker"`
			Version string `json:"version"`
		} `json:"node_info"`
		SyncInfo struct {
			LatestBlockHeight string `json:"latest_block_height"`
			LatestBlockTime   string `json:"latest_block_time"`
			CatchingUp        bool   `json:"catching_up"`
		} `json:"sync_info"`
		ValidatorInfo struct {
			Address     string `json:"address"`
			VotingPower string `json:"voting_power"`
		} `json:"validator_info"`
	} `json:"result"`
}

// NetInfoResponse represents the /net_info RPC response.
type NetInfoResponse struct {
	Result struct {
		Listening bool     `json:"listening"`
		Listeners []string `json:"listeners"`
		NPeers    string   `json:"n_peers"`
		Peers     []struct {
			NodeInfo struct {
				Moniker string `json:"moniker"`
			} `json:"node_info"`
			RemoteIP string `json:"remote_ip"`
		} `json:"peers"`
	} `json:"result"`
}

// Status fetches the node status from /status endpoint.
func (c *CometClient) Status() (*StatusResponse, error) {
	url := c.BaseURL + "/status"
	resp, err := c.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var status StatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return &status, nil
}

// NetInfo fetches network info from /net_info endpoint.
func (c *CometClient) NetInfo() (*NetInfoResponse, error) {
	url := c.BaseURL + "/net_info"
	resp, err := c.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var netInfo NetInfoResponse
	if err := json.Unmarshal(body, &netInfo); err != nil {
		return nil, fmt.Errorf("failed to parse net_info response: %w", err)
	}

	return &netInfo, nil
}

// Health checks if the node is responding.
func (c *CometClient) Health() error {
	url := c.BaseURL + "/health"
	resp, err := c.Client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("node unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

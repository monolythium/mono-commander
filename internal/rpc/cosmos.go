package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CosmosClient is a client for Cosmos REST API.
type CosmosClient struct {
	BaseURL string
	Client  *http.Client
}

// NewCosmosClient creates a new Cosmos REST client.
func NewCosmosClient(baseURL string) *CosmosClient {
	return &CosmosClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NodeInfoResponse represents the /cosmos/base/tendermint/v1beta1/node_info response.
type NodeInfoResponse struct {
	DefaultNodeInfo struct {
		Network  string `json:"network"`
		Moniker  string `json:"moniker"`
		Version  string `json:"version"`
		Channels string `json:"channels"`
	} `json:"default_node_info"`
	ApplicationVersion struct {
		Name      string `json:"name"`
		AppName   string `json:"app_name"`
		Version   string `json:"version"`
		GitCommit string `json:"git_commit"`
		GoVersion string `json:"go_version"`
	} `json:"application_version"`
}

// SyncingResponse represents the /cosmos/base/tendermint/v1beta1/syncing response.
type SyncingResponse struct {
	Syncing bool `json:"syncing"`
}

// LatestBlockResponse represents the /cosmos/base/tendermint/v1beta1/blocks/latest response.
type LatestBlockResponse struct {
	BlockID struct {
		Hash string `json:"hash"`
	} `json:"block_id"`
	Block struct {
		Header struct {
			ChainID string `json:"chain_id"`
			Height  string `json:"height"`
			Time    string `json:"time"`
		} `json:"header"`
	} `json:"block"`
}

// NodeInfo fetches node information.
func (c *CosmosClient) NodeInfo() (*NodeInfoResponse, error) {
	url := c.BaseURL + "/cosmos/base/tendermint/v1beta1/node_info"
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

	var info NodeInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse node_info response: %w", err)
	}

	return &info, nil
}

// Syncing checks if the node is syncing.
func (c *CosmosClient) Syncing() (*SyncingResponse, error) {
	url := c.BaseURL + "/cosmos/base/tendermint/v1beta1/syncing"
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

	var syncing SyncingResponse
	if err := json.Unmarshal(body, &syncing); err != nil {
		return nil, fmt.Errorf("failed to parse syncing response: %w", err)
	}

	return &syncing, nil
}

// LatestBlock fetches the latest block.
func (c *CosmosClient) LatestBlock() (*LatestBlockResponse, error) {
	url := c.BaseURL + "/cosmos/base/tendermint/v1beta1/blocks/latest"
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

	var block LatestBlockResponse
	if err := json.Unmarshal(body, &block); err != nil {
		return nil, fmt.Errorf("failed to parse latest block response: %w", err)
	}

	return &block, nil
}

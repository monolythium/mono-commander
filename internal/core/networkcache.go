package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheDir returns the cache directory for network configs
func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mono-commander", "networks"), nil
}

// CacheNetworkConfig caches a network configuration locally
func CacheNetworkConfig(config *NetworkConfig, ref string) error {
	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}

	networkDir := filepath.Join(cacheDir, config.NetworkName, ref)
	if err := os.MkdirAll(networkDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	configPath := filepath.Join(networkDir, "config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}

	// Write metadata
	meta := map[string]string{
		"cached_at": time.Now().UTC().Format(time.RFC3339),
		"ref":       ref,
	}
	metaPath := filepath.Join(networkDir, "meta.json")
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(metaPath, metaData, 0644)

	return nil
}

// LoadCachedConfig loads a cached network configuration
func LoadCachedConfig(network, ref string) (*NetworkConfig, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(cacheDir, network, ref, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no cached config for %s@%s", network, ref)
		}
		return nil, fmt.Errorf("failed to read cache: %w", err)
	}

	var config NetworkConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse cached config: %w", err)
	}

	return &config, nil
}

// GetNetworkConfigWithCache fetches config, using cache as fallback
func GetNetworkConfigWithCache(network, ref string) (*NetworkConfig, error) {
	// Try to fetch fresh config
	config, err := FetchNetworkConfig(network, ref)
	if err == nil {
		// Verify and cache
		if verifyErr := VerifyNetworkConfig(config); verifyErr != nil {
			return nil, verifyErr
		}
		CacheNetworkConfig(config, ref)
		return config, nil
	}

	// Fallback to cache
	cached, cacheErr := LoadCachedConfig(network, ref)
	if cacheErr != nil {
		return nil, fmt.Errorf("failed to fetch config (%v) and no cache available (%v)", err, cacheErr)
	}

	return cached, nil
}

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// GenesisDoc represents the minimal fields we need from genesis.json.
type GenesisDoc struct {
	ChainID string `json:"chain_id"`
}

// VerifyGenesisSHA256 verifies that the genesis file matches the expected hash.
func VerifyGenesisSHA256(genesisPath, expectedSHA string) error {
	data, err := os.ReadFile(genesisPath)
	if err != nil {
		return fmt.Errorf("failed to read genesis file: %w", err)
	}

	hash := sha256.Sum256(data)
	actual := hex.EncodeToString(hash[:])

	if actual != expectedSHA {
		return fmt.Errorf("genesis SHA256 mismatch: expected %s, got %s", expectedSHA, actual)
	}

	return nil
}

// ComputeSHA256 computes the SHA256 hash of a file.
func ComputeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// ParseGenesisChainID extracts the chain_id from a genesis file.
func ParseGenesisChainID(genesisPath string) (string, error) {
	data, err := os.ReadFile(genesisPath)
	if err != nil {
		return "", fmt.Errorf("failed to read genesis file: %w", err)
	}

	var doc GenesisDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("failed to parse genesis file: %w", err)
	}

	if doc.ChainID == "" {
		return "", fmt.Errorf("genesis file missing chain_id")
	}

	return doc.ChainID, nil
}

// WriteGenesis writes genesis data to the specified home directory.
func WriteGenesis(home string, data []byte, dryRun bool) (string, error) {
	configDir := filepath.Join(home, "config")
	genesisPath := filepath.Join(configDir, "genesis.json")

	if dryRun {
		return genesisPath, nil
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(genesisPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write genesis file: %w", err)
	}

	return genesisPath, nil
}

// ValidateGenesisData validates genesis JSON and returns the chain ID.
func ValidateGenesisData(data []byte) (string, error) {
	var doc GenesisDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("invalid genesis JSON: %w", err)
	}

	if doc.ChainID == "" {
		return "", fmt.Errorf("genesis missing chain_id field")
	}

	return doc.ChainID, nil
}

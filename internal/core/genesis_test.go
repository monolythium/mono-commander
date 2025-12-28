package core

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateGenesisData(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantChain string
		wantErr   bool
	}{
		{
			name:      "valid genesis",
			data:      `{"chain_id": "mono-sprint-1", "genesis_time": "2025-01-01T00:00:00Z"}`,
			wantChain: "mono-sprint-1",
			wantErr:   false,
		},
		{
			name:    "missing chain_id",
			data:    `{"genesis_time": "2025-01-01T00:00:00Z"}`,
			wantErr: true,
		},
		{
			name:    "empty chain_id",
			data:    `{"chain_id": "", "genesis_time": "2025-01-01T00:00:00Z"}`,
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
			chainID, err := ValidateGenesisData([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGenesisData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && chainID != tt.wantChain {
				t.Errorf("ValidateGenesisData() chainID = %v, want %v", chainID, tt.wantChain)
			}
		})
	}
}

func TestComputeSHA256(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")
	testData := []byte(`{"chain_id": "mono-sprint-1"}`)

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Compute expected hash
	expected := sha256.Sum256(testData)
	expectedHex := hex.EncodeToString(expected[:])

	// Test ComputeSHA256
	got, err := ComputeSHA256(testFile)
	if err != nil {
		t.Fatalf("ComputeSHA256() error = %v", err)
	}

	if got != expectedHex {
		t.Errorf("ComputeSHA256() = %v, want %v", got, expectedHex)
	}
}

func TestVerifyGenesisSHA256(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "genesis.json")
	testData := []byte(`{"chain_id": "mono-sprint-1"}`)

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Compute correct hash
	hash := sha256.Sum256(testData)
	correctSHA := hex.EncodeToString(hash[:])

	// Test with correct SHA
	err := VerifyGenesisSHA256(testFile, correctSHA)
	if err != nil {
		t.Errorf("VerifyGenesisSHA256() with correct SHA: error = %v", err)
	}

	// Test with incorrect SHA
	err = VerifyGenesisSHA256(testFile, "wronghash")
	if err == nil {
		t.Error("VerifyGenesisSHA256() with incorrect SHA: expected error")
	}

	// Test with non-existent file
	err = VerifyGenesisSHA256("/nonexistent/file", correctSHA)
	if err == nil {
		t.Error("VerifyGenesisSHA256() with non-existent file: expected error")
	}
}

func TestWriteGenesis(t *testing.T) {
	tmpDir := t.TempDir()
	testData := []byte(`{"chain_id": "mono-sprint-1"}`)

	// Test dry run
	path, err := WriteGenesis(tmpDir, testData, true)
	if err != nil {
		t.Errorf("WriteGenesis() dry run error = %v", err)
	}
	expectedPath := filepath.Join(tmpDir, "config", "genesis.json")
	if path != expectedPath {
		t.Errorf("WriteGenesis() path = %v, want %v", path, expectedPath)
	}

	// Verify file was NOT created in dry run
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("WriteGenesis() dry run created file")
	}

	// Test actual write
	path, err = WriteGenesis(tmpDir, testData, false)
	if err != nil {
		t.Errorf("WriteGenesis() error = %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("Failed to read written genesis: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("Written genesis content mismatch")
	}
}

func TestParseGenesisChainID(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "genesis.json")
	testData := []byte(`{"chain_id": "mono-sprint-1", "genesis_time": "2025-01-01T00:00:00Z"}`)

	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	chainID, err := ParseGenesisChainID(testFile)
	if err != nil {
		t.Errorf("ParseGenesisChainID() error = %v", err)
	}

	if chainID != "mono-sprint-1" {
		t.Errorf("ParseGenesisChainID() = %v, want mono-sprint-1", chainID)
	}
}

package walletgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestKeystoreCreation verifies keystore can be created and parsed
func TestKeystoreCreation(t *testing.T) {
	// Generate a keypair
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair failed: %v", err)
	}

	// Create keystore (use light params for faster tests)
	password := "testpassword123"
	ks, err := CreateKeystoreLight(kp, password)
	if err != nil {
		t.Fatalf("CreateKeystoreLight failed: %v", err)
	}

	// Verify keystore structure
	if ks.Version != 3 {
		t.Errorf("version = %d, want 3", ks.Version)
	}

	if ks.ID == "" {
		t.Error("ID should not be empty")
	}

	if ks.Address == "" {
		t.Error("Address should not be empty")
	}

	// Address should match keypair's EVM address (without 0x)
	expectedAddr := strings.ToLower(strings.TrimPrefix(kp.EVMAddress(), "0x"))
	if ks.Address != expectedAddr {
		t.Errorf("address = %s, want %s", ks.Address, expectedAddr)
	}

	// Verify crypto structure
	if ks.Crypto.Cipher != "aes-128-ctr" {
		t.Errorf("cipher = %s, want aes-128-ctr", ks.Crypto.Cipher)
	}

	if ks.Crypto.KDF != "scrypt" {
		t.Errorf("kdf = %s, want scrypt", ks.Crypto.KDF)
	}

	if ks.Crypto.CipherText == "" {
		t.Error("ciphertext should not be empty")
	}

	if ks.Crypto.CipherParams.IV == "" {
		t.Error("IV should not be empty")
	}

	if ks.Crypto.MAC == "" {
		t.Error("MAC should not be empty")
	}

	if ks.Crypto.KDFParams.Salt == "" {
		t.Error("salt should not be empty")
	}
}

// TestKeystoreSaveLoad verifies keystore can be saved and loaded
func TestKeystoreSaveLoad(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "walletgen-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate keypair and keystore
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair failed: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, "testpass")
	if err != nil {
		t.Fatalf("CreateKeystoreLight failed: %v", err)
	}

	// Save keystore
	filename := GenerateKeystoreFilename("test-wallet", kp.EVMAddress())
	path := filepath.Join(tempDir, filename)

	if err := SaveKeystore(ks, path); err != nil {
		t.Fatalf("SaveKeystore failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("keystore file was not created")
	}

	// Load keystore
	loaded, err := LoadKeystore(path)
	if err != nil {
		t.Fatalf("LoadKeystore failed: %v", err)
	}

	// Verify loaded data matches
	if loaded.Version != ks.Version {
		t.Errorf("loaded version = %d, want %d", loaded.Version, ks.Version)
	}

	if loaded.ID != ks.ID {
		t.Errorf("loaded ID = %s, want %s", loaded.ID, ks.ID)
	}

	if loaded.Address != ks.Address {
		t.Errorf("loaded address = %s, want %s", loaded.Address, ks.Address)
	}

	if loaded.Crypto.CipherText != ks.Crypto.CipherText {
		t.Error("ciphertext mismatch")
	}
}

// TestKeystoreJSON verifies keystore JSON is valid and parseable
func TestKeystoreJSON(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair failed: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, "testpass")
	if err != nil {
		t.Fatalf("CreateKeystoreLight failed: %v", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(ks, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal back
	var parsed KeystoreV3
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify critical fields
	if parsed.Version != 3 {
		t.Errorf("parsed version = %d, want 3", parsed.Version)
	}

	if parsed.Address != ks.Address {
		t.Errorf("parsed address = %s, want %s", parsed.Address, ks.Address)
	}

	// Log the JSON for inspection (no secrets here - ciphertext is encrypted)
	t.Logf("Keystore JSON:\n%s", string(data))
}

// TestGetKeystoreAddress verifies address extraction without decryption
func TestGetKeystoreAddress(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair failed: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, "testpass")
	if err != nil {
		t.Fatalf("CreateKeystoreLight failed: %v", err)
	}

	// Get EVM address
	evmAddr := GetKeystoreAddress(ks)
	expectedEVM := kp.EVMAddress()

	if strings.ToLower(evmAddr) != strings.ToLower(expectedEVM) {
		t.Errorf("GetKeystoreAddress = %s, want %s", evmAddr, expectedEVM)
	}

	// Get Bech32 address
	bech32Addr, err := GetKeystoreBech32Address(ks)
	if err != nil {
		t.Fatalf("GetKeystoreBech32Address failed: %v", err)
	}

	if !strings.HasPrefix(bech32Addr, "mono1") {
		t.Errorf("bech32 address should start with mono1, got %s", bech32Addr)
	}

	// Verify it matches keypair's bech32 address
	expectedBech32, _ := kp.Bech32Address()
	if bech32Addr != expectedBech32 {
		t.Errorf("GetKeystoreBech32Address = %s, want %s", bech32Addr, expectedBech32)
	}
}

// TestListKeystores verifies listing keystore files
func TestListKeystores(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "walletgen-list-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// List empty directory
	infos, err := ListKeystores(tempDir)
	if err != nil {
		t.Fatalf("ListKeystores on empty dir failed: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("expected 0 keystores, got %d", len(infos))
	}

	// Create some keystores
	for i := 0; i < 3; i++ {
		kp, err := GenerateKeypair()
		if err != nil {
			t.Fatalf("GenerateKeypair failed: %v", err)
		}

		ks, err := CreateKeystoreLight(kp, "testpass")
		if err != nil {
			t.Fatalf("CreateKeystoreLight failed: %v", err)
		}

		filename := GenerateKeystoreFilename("test", kp.EVMAddress())
		path := filepath.Join(tempDir, filename)

		if err := SaveKeystore(ks, path); err != nil {
			t.Fatalf("SaveKeystore failed: %v", err)
		}
	}

	// Create a non-keystore file
	nonKsPath := filepath.Join(tempDir, "not-a-keystore.json")
	os.WriteFile(nonKsPath, []byte("{}"), 0644)

	// List keystores
	infos, err = ListKeystores(tempDir)
	if err != nil {
		t.Fatalf("ListKeystores failed: %v", err)
	}

	if len(infos) != 3 {
		t.Errorf("expected 3 keystores, got %d", len(infos))
	}

	// Verify each has valid addresses
	for _, info := range infos {
		if !strings.HasPrefix(info.EVMAddress, "0x") {
			t.Errorf("EVM address should start with 0x: %s", info.EVMAddress)
		}
		if !strings.HasPrefix(info.Bech32Addr, "mono1") {
			t.Errorf("Bech32 address should start with mono1: %s", info.Bech32Addr)
		}
	}
}

// TestGenerateKeystoreFilename verifies filename generation
func TestGenerateKeystoreFilename(t *testing.T) {
	testCases := []struct {
		name    string
		evmAddr string
	}{
		{"test-wallet", "0x1234567890abcdef1234567890abcdef12345678"},
		{"", "0xabcdef1234567890abcdef1234567890abcdef12"},
		{"My Wallet!", "0x0000000000000000000000000000000000000000"},
	}

	for _, tc := range testCases {
		filename := GenerateKeystoreFilename(tc.name, tc.evmAddr)

		// Should have .json extension
		if !strings.HasSuffix(filename, ".json") {
			t.Errorf("filename should end with .json: %s", filename)
		}

		// Should start with UTC--
		if !strings.HasPrefix(filename, "UTC--") {
			t.Errorf("filename should start with UTC--: %s", filename)
		}

		// Should contain address (lowercase, no 0x)
		addrPart := strings.ToLower(strings.TrimPrefix(tc.evmAddr, "0x"))
		if !strings.Contains(filename, addrPart) {
			t.Errorf("filename should contain address: %s", filename)
		}
	}
}

// TestKeystoreNoSecrets ensures no private key logging
func TestKeystoreNoSecrets(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair failed: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, "testpass")
	if err != nil {
		t.Fatalf("CreateKeystoreLight failed: %v", err)
	}

	// Marshal to JSON
	data, err := json.Marshal(ks)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(data)

	// Get private key hex
	privKeyHex := kp.PrivateKeyHex()

	// Ensure private key is NOT in the JSON
	if strings.Contains(jsonStr, privKeyHex) {
		t.Error("SECURITY: private key found in keystore JSON!")
	}

	// Ensure password is not in the JSON
	if strings.Contains(jsonStr, "testpass") {
		t.Error("SECURITY: password found in keystore JSON!")
	}
}

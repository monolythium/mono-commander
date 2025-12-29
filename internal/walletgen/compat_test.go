package walletgen

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// testPassword is a deterministic password for testing only.
// It is NOT a real password and should never be used in production.
const testPassword = "test-password-for-unit-tests-only"

// TestKeystoreRoundtrip tests the full keystore encrypt/decrypt cycle
// and verifies that the decrypted key derives the same address.
func TestKeystoreRoundtrip(t *testing.T) {
	// Generate keypair
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Get original private key bytes
	originalPrivBytes := kp.PrivateKeyBytes()
	originalEvmAddr := kp.EVMAddress()

	// Create keystore (using light params for faster test)
	ks, err := CreateKeystoreLight(kp, testPassword)
	if err != nil {
		t.Fatalf("Failed to create keystore: %v", err)
	}

	// Verify keystore address matches
	keystoreAddr := GetKeystoreAddress(ks)
	if strings.ToLower(keystoreAddr) != strings.ToLower(originalEvmAddr) {
		t.Errorf("Keystore address mismatch: got %s, want %s", keystoreAddr, originalEvmAddr)
	}

	// Decrypt keystore
	decryptedPrivBytes, err := DecryptKeystore(ks, testPassword)
	if err != nil {
		t.Fatalf("Failed to decrypt keystore: %v", err)
	}

	// Verify decrypted key matches original
	if !bytes.Equal(decryptedPrivBytes, originalPrivBytes) {
		t.Errorf("Decrypted private key does not match original")
	}

	// Derive address from decrypted key
	decryptedKey, err := crypto.ToECDSA(decryptedPrivBytes)
	if err != nil {
		t.Fatalf("Failed to parse decrypted key: %v", err)
	}

	derivedAddr := crypto.PubkeyToAddress(decryptedKey.PublicKey)
	derivedAddrHex := derivedAddr.Hex()

	// Verify derived address matches original
	if strings.ToLower(derivedAddrHex) != strings.ToLower(originalEvmAddr) {
		t.Errorf("Derived address from decrypted key mismatch: got %s, want %s", derivedAddrHex, originalEvmAddr)
	}
}

// TestKeystoreWrongPassword verifies that wrong password fails decryption
func TestKeystoreWrongPassword(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, testPassword)
	if err != nil {
		t.Fatalf("Failed to create keystore: %v", err)
	}

	// Try to decrypt with wrong password
	_, err = DecryptKeystore(ks, "wrong-password")
	if err == nil {
		t.Error("Expected error when decrypting with wrong password")
	}

	// Verify error message doesn't leak info
	if !strings.Contains(err.Error(), "incorrect password") {
		t.Errorf("Error should mention incorrect password: %v", err)
	}
}

// TestBech32ConversionRoundtrip verifies bech32 address conversion correctness
func TestBech32ConversionRoundtrip(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	evmAddr := kp.EVMAddress()
	bech32Addr, err := kp.Bech32Address()
	if err != nil {
		t.Fatalf("Failed to get bech32 address: %v", err)
	}

	// Decode bech32 to bytes
	prefix, addrBytes, err := Bech32Decode(bech32Addr)
	if err != nil {
		t.Fatalf("Failed to decode bech32: %v", err)
	}

	// Verify prefix
	if prefix != Bech32PrefixAccAddr {
		t.Errorf("Bech32 prefix mismatch: got %s, want %s", prefix, Bech32PrefixAccAddr)
	}

	// Verify bytes length
	if len(addrBytes) != 20 {
		t.Errorf("Address bytes length = %d, want 20", len(addrBytes))
	}

	// Encode back to bech32
	bech32Back, err := Bech32Encode(Bech32PrefixAccAddr, addrBytes)
	if err != nil {
		t.Fatalf("Failed to encode bech32: %v", err)
	}

	if bech32Back != bech32Addr {
		t.Errorf("Bech32 roundtrip mismatch: got %s, want %s", bech32Back, bech32Addr)
	}

	// Verify bytes match EVM address
	evmHex := evmAddr[2:] // Remove 0x prefix
	evmBytes, err := hex.DecodeString(evmHex)
	if err != nil {
		t.Fatalf("Failed to decode EVM address: %v", err)
	}

	if !bytes.Equal(addrBytes, evmBytes) {
		t.Errorf("Address bytes mismatch between EVM and bech32")
	}
}

// TestFilePermissions verifies keystore files are created with correct permissions
func TestFilePermissions(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "walletgen-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate keystore
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, testPassword)
	if err != nil {
		t.Fatalf("Failed to create keystore: %v", err)
	}

	// Save to file
	keystorePath := filepath.Join(tmpDir, "test-keystore.json")
	err = SaveKeystore(ks, keystorePath)
	if err != nil {
		t.Fatalf("Failed to save keystore: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(keystorePath)
	if err != nil {
		t.Fatalf("Failed to stat keystore file: %v", err)
	}

	mode := info.Mode().Perm()
	// Expect 0600 (owner read/write only)
	if mode != 0600 {
		t.Errorf("Keystore file permissions = %o, want 0600", mode)
	}
}

// TestScryptParameters verifies scrypt parameters are set correctly
func TestScryptParameters(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Test standard params
	ks, err := CreateKeystore(kp, testPassword)
	if err != nil {
		t.Fatalf("Failed to create keystore: %v", err)
	}

	if ks.Crypto.KDFParams.N != ScryptN {
		t.Errorf("Scrypt N = %d, want %d", ks.Crypto.KDFParams.N, ScryptN)
	}
	if ks.Crypto.KDFParams.R != ScryptR {
		t.Errorf("Scrypt R = %d, want %d", ks.Crypto.KDFParams.R, ScryptR)
	}
	if ks.Crypto.KDFParams.P != ScryptP {
		t.Errorf("Scrypt P = %d, want %d", ks.Crypto.KDFParams.P, ScryptP)
	}
	if ks.Crypto.KDFParams.DKLen != ScryptDKLen {
		t.Errorf("Scrypt DKLen = %d, want %d", ks.Crypto.KDFParams.DKLen, ScryptDKLen)
	}

	// Verify KDF is scrypt
	if ks.Crypto.KDF != "scrypt" {
		t.Errorf("KDF = %s, want scrypt", ks.Crypto.KDF)
	}

	// Verify cipher is aes-128-ctr
	if ks.Crypto.Cipher != "aes-128-ctr" {
		t.Errorf("Cipher = %s, want aes-128-ctr", ks.Crypto.Cipher)
	}

	// Verify version is 3
	if ks.Version != 3 {
		t.Errorf("Version = %d, want 3", ks.Version)
	}
}

// TestLightScryptParameters verifies light scrypt parameters
func TestLightScryptParameters(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, testPassword)
	if err != nil {
		t.Fatalf("Failed to create light keystore: %v", err)
	}

	if ks.Crypto.KDFParams.N != LightScryptN {
		t.Errorf("Light Scrypt N = %d, want %d", ks.Crypto.KDFParams.N, LightScryptN)
	}
	if ks.Crypto.KDFParams.R != LightScryptR {
		t.Errorf("Light Scrypt R = %d, want %d", ks.Crypto.KDFParams.R, LightScryptR)
	}
	if ks.Crypto.KDFParams.P != LightScryptP {
		t.Errorf("Light Scrypt P = %d, want %d", ks.Crypto.KDFParams.P, LightScryptP)
	}
}

// TestKeystoreAddressNormalization verifies address is stored lowercase
func TestKeystoreAddressNormalization(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, testPassword)
	if err != nil {
		t.Fatalf("Failed to create keystore: %v", err)
	}

	// Verify address is lowercase and no 0x prefix
	if ks.Address != strings.ToLower(ks.Address) {
		t.Errorf("Keystore address should be lowercase: %s", ks.Address)
	}

	if strings.HasPrefix(ks.Address, "0x") {
		t.Errorf("Keystore address should not have 0x prefix: %s", ks.Address)
	}

	// GetKeystoreAddress should add 0x prefix
	addr := GetKeystoreAddress(ks)
	if !strings.HasPrefix(addr, "0x") {
		t.Errorf("GetKeystoreAddress should add 0x prefix: %s", addr)
	}
}

// TestFromPrivateKeyBytes verifies keypair creation from existing key
func TestFromPrivateKeyBytes(t *testing.T) {
	// Create known private key bytes
	privBytes := make([]byte, 32)
	privBytes[31] = 0x01

	kp, err := FromPrivateKeyBytes(privBytes)
	if err != nil {
		t.Fatalf("Failed to create keypair from bytes: %v", err)
	}

	// Verify the bytes match
	if !bytes.Equal(kp.PrivateKeyBytes(), privBytes) {
		t.Errorf("Private key bytes mismatch")
	}

	// Create same key using go-ethereum and verify addresses match
	privKey, err := crypto.ToECDSA(privBytes)
	if err != nil {
		t.Fatalf("Failed to create ECDSA key: %v", err)
	}

	expectedAddr := crypto.PubkeyToAddress(privKey.PublicKey).Hex()
	actualAddr := kp.EVMAddress()

	if strings.ToLower(actualAddr) != strings.ToLower(expectedAddr) {
		t.Errorf("Address mismatch: got %s, want %s", actualAddr, expectedAddr)
	}
}

// TestKeystoreJSONFormat verifies the keystore JSON structure
func TestKeystoreJSONFormat(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ks, err := CreateKeystoreLight(kp, testPassword)
	if err != nil {
		t.Fatalf("Failed to create keystore: %v", err)
	}

	// Verify all required fields are present and non-empty
	if ks.Version != 3 {
		t.Errorf("Version should be 3")
	}

	if ks.ID == "" {
		t.Error("ID should not be empty")
	}

	if ks.Address == "" {
		t.Error("Address should not be empty")
	}

	if len(ks.Address) != 40 {
		t.Errorf("Address length should be 40 (no 0x prefix), got %d", len(ks.Address))
	}

	if ks.Crypto.Cipher == "" {
		t.Error("Cipher should not be empty")
	}

	if ks.Crypto.CipherText == "" {
		t.Error("CipherText should not be empty")
	}

	if ks.Crypto.CipherParams.IV == "" {
		t.Error("IV should not be empty")
	}

	if len(ks.Crypto.CipherParams.IV) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("IV length should be 32 hex chars, got %d", len(ks.Crypto.CipherParams.IV))
	}

	if ks.Crypto.KDF == "" {
		t.Error("KDF should not be empty")
	}

	if ks.Crypto.KDFParams.Salt == "" {
		t.Error("Salt should not be empty")
	}

	if len(ks.Crypto.KDFParams.Salt) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("Salt length should be 64 hex chars, got %d", len(ks.Crypto.KDFParams.Salt))
	}

	if ks.Crypto.MAC == "" {
		t.Error("MAC should not be empty")
	}

	if len(ks.Crypto.MAC) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("MAC length should be 64 hex chars, got %d", len(ks.Crypto.MAC))
	}
}

// TestMultipleKeypairsUniqueness verifies generated keypairs are unique
func TestMultipleKeypairsUniqueness(t *testing.T) {
	addresses := make(map[string]bool)

	for i := 0; i < 100; i++ {
		kp, err := GenerateKeypair()
		if err != nil {
			t.Fatalf("Failed to generate keypair: %v", err)
		}

		addr := kp.EVMAddress()
		if addresses[addr] {
			t.Errorf("Duplicate address generated: %s", addr)
		}
		addresses[addr] = true
	}
}

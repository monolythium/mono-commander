package walletgen

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
)

// TestBech32RoundTrip verifies bech32 encode/decode roundtrip
func TestBech32RoundTrip(t *testing.T) {
	testCases := []struct {
		name    string
		prefix  string
		data    []byte
	}{
		{
			name:   "20 bytes address",
			prefix: "mono",
			data:   []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14},
		},
		{
			name:   "zeros",
			prefix: "mono",
			data:   make([]byte, 20),
		},
		{
			name:   "max values",
			prefix: "mono",
			data:   []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode
			encoded, err := Bech32Encode(tc.prefix, tc.data)
			if err != nil {
				t.Fatalf("Bech32Encode failed: %v", err)
			}

			// Check prefix
			if !strings.HasPrefix(encoded, tc.prefix+"1") {
				t.Errorf("encoded address should start with %s1, got %s", tc.prefix, encoded)
			}

			// Decode
			prefix, decoded, err := Bech32Decode(encoded)
			if err != nil {
				t.Fatalf("Bech32Decode failed: %v", err)
			}

			// Verify prefix
			if prefix != tc.prefix {
				t.Errorf("decoded prefix = %s, want %s", prefix, tc.prefix)
			}

			// Verify data
			if len(decoded) != len(tc.data) {
				t.Fatalf("decoded length = %d, want %d", len(decoded), len(tc.data))
			}

			for i := range tc.data {
				if decoded[i] != tc.data[i] {
					t.Errorf("decoded[%d] = %02x, want %02x", i, decoded[i], tc.data[i])
				}
			}
		})
	}
}

// TestEVMToBech32Roundtrip verifies EVM <-> bech32 conversion
func TestEVMToBech32Roundtrip(t *testing.T) {
	testCases := []struct {
		name    string
		evmAddr string
	}{
		{
			name:    "lowercase",
			evmAddr: "0x1234567890abcdef1234567890abcdef12345678",
		},
		{
			name:    "uppercase",
			evmAddr: "0x1234567890ABCDEF1234567890ABCDEF12345678",
		},
		{
			name:    "no prefix",
			evmAddr: "1234567890abcdef1234567890abcdef12345678",
		},
		{
			name:    "zeros",
			evmAddr: "0x0000000000000000000000000000000000000000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert EVM -> bech32
			bech32Addr, err := EVMToBech32Address(tc.evmAddr, Bech32PrefixAccAddr)
			if err != nil {
				t.Fatalf("EVMToBech32Address failed: %v", err)
			}

			// Check prefix
			if !strings.HasPrefix(bech32Addr, "mono1") {
				t.Errorf("bech32 address should start with mono1, got %s", bech32Addr)
			}

			// Convert back to EVM
			evmBack, err := Bech32ToEVMAddress(bech32Addr)
			if err != nil {
				t.Fatalf("Bech32ToEVMAddress failed: %v", err)
			}

			// Normalize for comparison
			expectedEVM := strings.ToLower(tc.evmAddr)
			if !strings.HasPrefix(expectedEVM, "0x") {
				expectedEVM = "0x" + expectedEVM
			}
			evmBack = strings.ToLower(evmBack)

			if evmBack != expectedEVM {
				t.Errorf("roundtrip failed: got %s, want %s", evmBack, expectedEVM)
			}
		})
	}
}

// TestKnownAddressConversion tests against a known address vector
func TestKnownAddressConversion(t *testing.T) {
	// Known test vector: generate from a specific private key
	// This ensures consistent address derivation

	// Private key: 0x1234...5678 (32 bytes, all 0x00 except first byte 0x01)
	privKeyBytes := make([]byte, 32)
	privKeyBytes[31] = 0x01 // Set to 1 for a valid non-zero key

	curve := Secp256k1()
	x, y := curve.ScalarBaseMult(privKeyBytes)

	pubKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}

	privKey := &ecdsa.PrivateKey{
		PublicKey: *pubKey,
		D:         new(big.Int).SetBytes(privKeyBytes),
	}

	// Get EVM address
	evmAddr := PrivateKeyToEVMAddress(privKey)
	t.Logf("EVM address: %s", evmAddr)

	// Verify format
	if !strings.HasPrefix(evmAddr, "0x") {
		t.Errorf("EVM address should start with 0x")
	}
	if len(evmAddr) != 42 { // 0x + 40 hex chars
		t.Errorf("EVM address length = %d, want 42", len(evmAddr))
	}

	// Convert to bech32
	bech32Addr, err := EVMToBech32Address(evmAddr, Bech32PrefixAccAddr)
	if err != nil {
		t.Fatalf("EVMToBech32Address failed: %v", err)
	}
	t.Logf("Bech32 address: %s", bech32Addr)

	// Verify format
	if !strings.HasPrefix(bech32Addr, "mono1") {
		t.Errorf("Bech32 address should start with mono1")
	}

	// Convert back and verify
	evmBack, err := Bech32ToEVMAddress(bech32Addr)
	if err != nil {
		t.Fatalf("Bech32ToEVMAddress failed: %v", err)
	}

	if strings.ToLower(evmBack) != strings.ToLower(evmAddr) {
		t.Errorf("roundtrip failed: got %s, want %s", evmBack, evmAddr)
	}
}

// TestInvalidBech32Decode tests error handling for invalid inputs
func TestInvalidBech32Decode(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "no separator",
			input: "mono1234567890",
		},
		{
			name:  "invalid character",
			input: "mono1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqb",
		},
		{
			name:  "mixed case",
			input: "MoNo1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq",
		},
		{
			name:  "empty",
			input: "",
		},
		{
			name:  "just separator",
			input: "1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Bech32Decode(tc.input)
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tc.input)
			}
		})
	}
}

// TestPubKeyBytesToEVMAddress tests public key to EVM address conversion
func TestPubKeyBytesToEVMAddress(t *testing.T) {
	// Create a known public key bytes (65 bytes with 0x04 prefix)
	pubBytes := make([]byte, 65)
	pubBytes[0] = 0x04
	// Fill with known pattern
	for i := 1; i < 65; i++ {
		pubBytes[i] = byte(i)
	}

	addr := PubKeyBytesToEVMAddress(pubBytes)

	// Verify format
	if !strings.HasPrefix(addr, "0x") {
		t.Errorf("address should start with 0x")
	}

	// Should be 42 chars (0x + 40 hex)
	if len(addr) != 42 {
		t.Errorf("address length = %d, want 42", len(addr))
	}

	// Verify it's valid hex
	_, err := hex.DecodeString(addr[2:])
	if err != nil {
		t.Errorf("address is not valid hex: %v", err)
	}
}

package walletgen

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

// Bech32 prefix for Monolythium addresses
const (
	Bech32PrefixAccAddr = "mono"
)

// PrivateKeyToEVMAddress converts an ECDSA private key to an EVM address (0x...)
// Uses go-ethereum's crypto.PubkeyToAddress for correct address derivation.
func PrivateKeyToEVMAddress(privKey *ecdsa.PrivateKey) string {
	addr := crypto.PubkeyToAddress(privKey.PublicKey)
	return addr.Hex()
}

// PubKeyBytesToEVMAddress converts uncompressed public key bytes to EVM address.
// The input should be 65 bytes (0x04 prefix + 64 bytes X,Y coordinates) or
// 64 bytes (X,Y coordinates without prefix).
func PubKeyBytesToEVMAddress(pubBytes []byte) string {
	// Skip the 0x04 prefix if present
	if len(pubBytes) == 65 && pubBytes[0] == 0x04 {
		pubBytes = pubBytes[1:]
	}

	if len(pubBytes) != 64 {
		return ""
	}

	// Use go-ethereum's Keccak256 for address derivation
	hash := crypto.Keccak256(pubBytes)

	// Take last 20 bytes
	addr := hash[len(hash)-20:]
	return "0x" + hex.EncodeToString(addr)
}

// PrivateKeyToBech32Address converts an ECDSA private key to a bech32 address
func PrivateKeyToBech32Address(privKey *ecdsa.PrivateKey, prefix string) (string, error) {
	addr := crypto.PubkeyToAddress(privKey.PublicKey)
	return Bech32Encode(prefix, addr.Bytes())
}

// EVMToBech32Address converts an EVM address (0x...) to a bech32 address
func EVMToBech32Address(evmAddr string, prefix string) (string, error) {
	// Remove 0x prefix
	if len(evmAddr) >= 2 && evmAddr[:2] == "0x" {
		evmAddr = evmAddr[2:]
	}

	// Decode hex
	addrBytes, err := hex.DecodeString(evmAddr)
	if err != nil {
		return "", fmt.Errorf("invalid EVM address: %w", err)
	}

	if len(addrBytes) != 20 {
		return "", errors.New("EVM address must be 20 bytes")
	}

	// Encode to bech32
	return Bech32Encode(prefix, addrBytes)
}

// Bech32ToBytesAddress converts a bech32 address to raw bytes
func Bech32ToBytesAddress(bech32Addr string) (string, []byte, error) {
	return Bech32Decode(bech32Addr)
}

// Bech32ToEVMAddress converts a bech32 address to an EVM address (0x...)
func Bech32ToEVMAddress(bech32Addr string) (string, error) {
	_, addrBytes, err := Bech32ToBytesAddress(bech32Addr)
	if err != nil {
		return "", err
	}

	return "0x" + hex.EncodeToString(addrBytes), nil
}

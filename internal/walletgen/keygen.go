// Package walletgen provides secure wallet/keypair generation for Monolythium.
// Uses go-ethereum's battle-tested crypto library for key generation and address derivation.
package walletgen

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

// Keypair holds the generated private/public key pair.
// The private key is kept in memory and NEVER logged.
type Keypair struct {
	privateKey *ecdsa.PrivateKey
}

// GenerateKeypair creates a new secp256k1 keypair using go-ethereum's crypto library.
// This uses crypto/rand internally for secure random number generation.
// The private key is never exposed directly - use methods to derive addresses.
func GenerateKeypair() (*Keypair, error) {
	privKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	return &Keypair{
		privateKey: privKey,
	}, nil
}

// FromPrivateKeyBytes creates a Keypair from raw private key bytes.
// WARNING: Use with extreme caution - only for testing or key recovery.
func FromPrivateKeyBytes(privBytes []byte) (*Keypair, error) {
	privKey, err := crypto.ToECDSA(privBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return &Keypair{privateKey: privKey}, nil
}

// EVMAddress returns the 0x-prefixed EVM address derived from this keypair.
// Uses go-ethereum's PubkeyToAddress for correct address derivation.
func (k *Keypair) EVMAddress() string {
	addr := crypto.PubkeyToAddress(k.privateKey.PublicKey)
	return addr.Hex()
}

// Bech32Address returns the mono1-prefixed bech32 address derived from this keypair.
func (k *Keypair) Bech32Address() (string, error) {
	addr := crypto.PubkeyToAddress(k.privateKey.PublicKey)
	return Bech32Encode(Bech32PrefixAccAddr, addr.Bytes())
}

// PrivateKeyBytes returns the raw private key bytes (32 bytes, left-padded).
// WARNING: This exposes the private key - use with extreme caution.
// Only call when the user explicitly requests it with confirmation.
func (k *Keypair) PrivateKeyBytes() []byte {
	return crypto.FromECDSA(k.privateKey)
}

// PrivateKeyHex returns the private key as a hex string (64 chars, no 0x prefix).
// WARNING: This exposes the private key - use with extreme caution.
// Only call when the user explicitly requests it with confirmation.
func (k *Keypair) PrivateKeyHex() string {
	return fmt.Sprintf("%064x", crypto.FromECDSA(k.privateKey))
}

// PrivateKey returns the underlying ECDSA private key.
// WARNING: This exposes the private key - use with extreme caution.
func (k *Keypair) PrivateKey() *ecdsa.PrivateKey {
	return k.privateKey
}

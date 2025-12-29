// Package walletgen provides secure wallet/keypair generation for Monolythium.
package walletgen

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
)

// Keypair holds the generated private/public key pair.
// The private key is kept in memory and NEVER logged.
type Keypair struct {
	privateKey *ecdsa.PrivateKey
}

// GenerateKeypair creates a new secp256k1 keypair using OS CSPRNG.
// The private key is never exposed directly - use methods to derive addresses.
func GenerateKeypair() (*Keypair, error) {
	privKey, err := ecdsa.GenerateKey(Secp256k1(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	return &Keypair{
		privateKey: privKey,
	}, nil
}

// EVMAddress returns the 0x-prefixed EVM address derived from this keypair.
func (k *Keypair) EVMAddress() string {
	return PrivateKeyToEVMAddress(k.privateKey)
}

// Bech32Address returns the mono1-prefixed bech32 address derived from this keypair.
func (k *Keypair) Bech32Address() (string, error) {
	return PrivateKeyToBech32Address(k.privateKey, Bech32PrefixAccAddr)
}

// PrivateKeyBytes returns the raw private key bytes.
// WARNING: This exposes the private key - use with extreme caution.
// Only call when the user explicitly requests it with confirmation.
func (k *Keypair) PrivateKeyBytes() []byte {
	return k.privateKey.D.Bytes()
}

// PrivateKeyHex returns the private key as a hex string.
// WARNING: This exposes the private key - use with extreme caution.
// Only call when the user explicitly requests it with confirmation.
func (k *Keypair) PrivateKeyHex() string {
	bytes := k.privateKey.D.Bytes()
	// Ensure 32-byte length (left-pad with zeros if needed)
	padded := make([]byte, 32)
	copy(padded[32-len(bytes):], bytes)
	return fmt.Sprintf("%064x", padded)
}

// PrivateKey returns the underlying ECDSA private key.
// WARNING: This exposes the private key - use with extreme caution.
func (k *Keypair) PrivateKey() *ecdsa.PrivateKey {
	return k.privateKey
}

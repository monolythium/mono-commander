package walletgen

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/scrypt"
)

// KeystoreV3 represents an Ethereum-compatible keystore v3 file
type KeystoreV3 struct {
	Version int      `json:"version"`
	ID      string   `json:"id"`
	Address string   `json:"address"`
	Crypto  CryptoV3 `json:"crypto"`
}

// CryptoV3 holds the encrypted key data
type CryptoV3 struct {
	Cipher       string         `json:"cipher"`
	CipherText   string         `json:"ciphertext"`
	CipherParams CipherParamsV3 `json:"cipherparams"`
	KDF          string         `json:"kdf"`
	KDFParams    ScryptParamsV3 `json:"kdfparams"`
	MAC          string         `json:"mac"`
}

// CipherParamsV3 holds the AES-128-CTR IV
type CipherParamsV3 struct {
	IV string `json:"iv"`
}

// ScryptParamsV3 holds scrypt KDF parameters
type ScryptParamsV3 struct {
	N     int    `json:"n"`
	R     int    `json:"r"`
	P     int    `json:"p"`
	DKLen int    `json:"dklen"`
	Salt  string `json:"salt"`
}

// Default scrypt parameters (matches go-ethereum defaults)
// These are intentionally strong - keystore encryption should be slow to prevent brute force.
const (
	ScryptN     = 262144 // 2^18
	ScryptR     = 8
	ScryptP     = 1
	ScryptDKLen = 32
)

// Light scrypt parameters (for testing or low-memory systems)
const (
	LightScryptN = 4096 // 2^12
	LightScryptR = 8
	LightScryptP = 6
)

// CreateKeystore creates an encrypted keystore v3 JSON for the given keypair
func CreateKeystore(kp *Keypair, password string) (*KeystoreV3, error) {
	return createKeystoreWithParams(kp, password, ScryptN, ScryptR, ScryptP)
}

// CreateKeystoreLight creates a keystore with lighter parameters (faster but less secure)
func CreateKeystoreLight(kp *Keypair, password string) (*KeystoreV3, error) {
	return createKeystoreWithParams(kp, password, LightScryptN, LightScryptR, LightScryptP)
}

func createKeystoreWithParams(kp *Keypair, password string, n, r, p int) (*KeystoreV3, error) {
	// Get private key bytes (always 32 bytes from go-ethereum's crypto.FromECDSA)
	privBytes := kp.PrivateKeyBytes()

	// Generate random salt (32 bytes)
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key using scrypt
	derivedKey, err := scrypt.Key([]byte(password), salt, n, r, p, ScryptDKLen)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	// Split derived key: first 16 bytes for encryption, bytes 16-32 for MAC
	encKey := derivedKey[:16]

	// Generate random IV (16 bytes for AES-128-CTR)
	iv := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Encrypt private key with AES-128-CTR
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	ciphertext := make([]byte, len(privBytes))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, privBytes)

	// Calculate MAC: keccak256(derivedKey[16:32] + ciphertext)
	// Using go-ethereum's Keccak256 for consistency
	macData := append(derivedKey[16:32], ciphertext...)
	mac := crypto.Keccak256(macData)

	// Generate UUID
	uuid, err := generateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	// Get address without 0x prefix
	addr := kp.EVMAddress()
	if strings.HasPrefix(addr, "0x") {
		addr = addr[2:]
	}

	return &KeystoreV3{
		Version: 3,
		ID:      uuid,
		Address: strings.ToLower(addr),
		Crypto: CryptoV3{
			Cipher:     "aes-128-ctr",
			CipherText: hex.EncodeToString(ciphertext),
			CipherParams: CipherParamsV3{
				IV: hex.EncodeToString(iv),
			},
			KDF: "scrypt",
			KDFParams: ScryptParamsV3{
				N:     n,
				R:     r,
				P:     p,
				DKLen: ScryptDKLen,
				Salt:  hex.EncodeToString(salt),
			},
			MAC: hex.EncodeToString(mac),
		},
	}, nil
}

// DecryptKeystore decrypts a keystore and returns the private key bytes.
// This is used for compatibility testing - the password is required.
// WARNING: Returns raw private key bytes - handle with extreme care.
func DecryptKeystore(ks *KeystoreV3, password string) ([]byte, error) {
	if ks.Crypto.KDF != "scrypt" {
		return nil, errors.New("unsupported KDF: only scrypt is supported")
	}
	if ks.Crypto.Cipher != "aes-128-ctr" {
		return nil, errors.New("unsupported cipher: only aes-128-ctr is supported")
	}

	// Decode hex values
	salt, err := hex.DecodeString(ks.Crypto.KDFParams.Salt)
	if err != nil {
		return nil, fmt.Errorf("invalid salt: %w", err)
	}

	ciphertext, err := hex.DecodeString(ks.Crypto.CipherText)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext: %w", err)
	}

	iv, err := hex.DecodeString(ks.Crypto.CipherParams.IV)
	if err != nil {
		return nil, fmt.Errorf("invalid IV: %w", err)
	}

	storedMAC, err := hex.DecodeString(ks.Crypto.MAC)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC: %w", err)
	}

	// Derive key using scrypt
	n := ks.Crypto.KDFParams.N
	r := ks.Crypto.KDFParams.R
	p := ks.Crypto.KDFParams.P
	dkLen := ks.Crypto.KDFParams.DKLen

	derivedKey, err := scrypt.Key([]byte(password), salt, n, r, p, dkLen)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	// Verify MAC
	macData := append(derivedKey[16:32], ciphertext...)
	calculatedMAC := crypto.Keccak256(macData)

	if subtle.ConstantTimeCompare(storedMAC, calculatedMAC) != 1 {
		return nil, errors.New("incorrect password or corrupted keystore")
	}

	// Decrypt
	encKey := derivedKey[:16]
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	privBytes := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(privBytes, ciphertext)

	return privBytes, nil
}

// SaveKeystore saves a keystore to a file
func SaveKeystore(ks *KeystoreV3, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(ks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keystore: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write keystore: %w", err)
	}

	return nil
}

// LoadKeystore loads a keystore from a file (without decrypting)
func LoadKeystore(path string) (*KeystoreV3, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read keystore: %w", err)
	}

	var ks KeystoreV3
	if err := json.Unmarshal(data, &ks); err != nil {
		return nil, fmt.Errorf("failed to parse keystore: %w", err)
	}

	// Validate it's actually a keystore v3
	if ks.Version != 3 || ks.Address == "" || ks.Crypto.CipherText == "" {
		return nil, fmt.Errorf("invalid keystore v3 format")
	}

	return &ks, nil
}

// GetKeystoreAddress returns the EVM address from a keystore without decryption
func GetKeystoreAddress(ks *KeystoreV3) string {
	addr := ks.Address
	if !strings.HasPrefix(addr, "0x") {
		addr = "0x" + addr
	}
	return addr
}

// GetKeystoreBech32Address derives the bech32 address from a keystore
func GetKeystoreBech32Address(ks *KeystoreV3) (string, error) {
	evmAddr := GetKeystoreAddress(ks)
	return EVMToBech32Address(evmAddr, Bech32PrefixAccAddr)
}

// GetDefaultWalletDir returns the default wallet directory path
func GetDefaultWalletDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".mono-commander", "wallets"), nil
}

// GenerateKeystoreFilename generates a filename for a new keystore
func GenerateKeystoreFilename(name string, evmAddr string) string {
	// Clean the name
	name = strings.TrimSpace(name)
	if name == "" {
		name = "wallet"
	}

	// Remove invalid characters
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, name)

	// Format: UTC--<timestamp>--<name>--<address>.json
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05.000000000Z")
	addr := strings.TrimPrefix(strings.ToLower(evmAddr), "0x")

	return fmt.Sprintf("UTC--%s--%s--%s.json", timestamp, name, addr)
}

// ListKeystores lists all keystore files in a directory
func ListKeystores(dir string) ([]KeystoreInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []KeystoreInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var result []KeystoreInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(dir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		ks, err := LoadKeystore(path)
		if err != nil {
			// Not a valid keystore, skip
			continue
		}

		evmAddr := GetKeystoreAddress(ks)
		bech32Addr, _ := GetKeystoreBech32Address(ks)

		result = append(result, KeystoreInfo{
			Filename:   name,
			Path:       path,
			EVMAddress: evmAddr,
			Bech32Addr: bech32Addr,
			CreatedAt:  info.ModTime(),
		})
	}

	return result, nil
}

// KeystoreInfo holds metadata about a keystore file
type KeystoreInfo struct {
	Filename   string    `json:"filename"`
	Path       string    `json:"path"`
	EVMAddress string    `json:"evm_address"`
	Bech32Addr string    `json:"bech32_address"`
	CreatedAt  time.Time `json:"created_at"`
}

// generateUUID generates a random UUID v4
func generateUUID() (string, error) {
	uuid := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, uuid); err != nil {
		return "", err
	}

	// Set version (4) and variant (RFC 4122)
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	), nil
}

# Wallet Generator Compatibility

This document describes the wallet generator's compatibility with Ethereum tooling and how to verify address consistency.

## Keystore Format

The wallet generator creates **Ethereum Keystore V3** files, compatible with:
- MetaMask (browser extension supports JSON file import)
- Geth/go-ethereum
- MyEtherWallet
- Other standard Ethereum wallets

**MetaMask Import**: The MetaMask browser extension can import V3 keystore files via Account > Import Account > JSON File. You select the keystore file and enter your password. See MetaMask's documentation on importing accounts for details.

### File Structure

```json
{
  "version": 3,
  "id": "<uuid-v4>",
  "address": "<40-char-hex-no-0x>",
  "crypto": {
    "cipher": "aes-128-ctr",
    "ciphertext": "<encrypted-private-key>",
    "cipherparams": {
      "iv": "<16-byte-hex-iv>"
    },
    "kdf": "scrypt",
    "kdfparams": {
      "n": 262144,
      "r": 8,
      "p": 1,
      "dklen": 32,
      "salt": "<32-byte-hex-salt>"
    },
    "mac": "<keccak256-mac>"
  }
}
```

### Scrypt Parameters

The default scrypt parameters match go-ethereum's defaults:
- **N = 262144** (2^18) - Memory cost
- **R = 8** - Block size
- **P = 1** - Parallelization
- **DKLen = 32** - Derived key length

These parameters are **intentionally strong** to prevent brute-force attacks. Key derivation takes ~1-2 seconds on modern hardware.

## Address Derivation

### Implementation (current as of 2025-12-28)

The wallet generator uses:
- `github.com/ethereum/go-ethereum/crypto` - Key generation and address derivation
- `crypto.GenerateKey()` - Uses crypto/rand (OS CSPRNG) internally
- `crypto.PubkeyToAddress()` - Standard Ethereum address derivation
- `crypto.Keccak256()` - For MAC calculation
- `golang.org/x/crypto/scrypt` - Key derivation function
- Standard library `crypto/aes` - AES-128-CTR encryption

### EVM Address (0x...)

The EVM address is derived using the standard Ethereum method:
1. Generate secp256k1 private key
2. Derive public key (uncompressed, 64 bytes)
3. Keccak256 hash of public key bytes
4. Take last 20 bytes as address

### Bech32 Address (mono1...)

The bech32 address is derived from the same 20-byte address:
1. Get EVM address bytes (20 bytes)
2. Encode using bech32 with "mono" prefix
3. Result: `mono1...` format

**Both addresses represent the same account** - they share the same 20-byte address bytes.

## Verifying Address Consistency

### Method 1: Using the CLI

```bash
# Generate a wallet
monoctl wallet generate --name test-wallet

# View wallet info
monoctl wallet info --file ~/.mono-commander/wallets/UTC--<timestamp>--test-wallet--<address>.json
```

Output shows both addresses:
```
EVM Address:    0x1234567890abcdef1234567890abcdef12345678
Bech32 Address: mono1zg69g5ys9ue3q...
```

### Method 2: Verify Bech32 Conversion

To verify the addresses are consistent, you can manually check:

1. Take the EVM address without `0x`: `1234567890abcdef1234567890abcdef12345678`
2. Decode the bech32 address to get the 20-byte data
3. Compare - they should be identical

### Method 3: Using monod

```bash
# Convert EVM to bech32
monod debug addr 0x1234567890abcdef1234567890abcdef12345678
```

## Security Considerations

### What NOT to Do

- **NEVER** paste private keys into chat, emails, or public forums
- **NEVER** share keystore files without password protection
- **NEVER** use weak passwords (less than 12 characters)
- **NEVER** store passwords in plain text files

### Keystore File Permissions

Keystore files are created with `0600` permissions (owner read/write only).

### Private Key Display

By default, private keys are **never displayed**. To show a private key:

```bash
monoctl wallet generate --name test --show-private-key --insecure-show=true
```

Both flags are required as a safety measure.

## Testing Compatibility

### Automated Tests

The wallet generator includes comprehensive compatibility tests:

```bash
cd mono-commander
go test ./internal/walletgen/... -v
```

Key tests:
- `TestKeystoreRoundtrip` - Encrypt, decrypt, verify address
- `TestBech32ConversionRoundtrip` - Verify address bytes match
- `TestFilePermissions` - Verify 0600 permissions
- `TestScryptParameters` - Verify KDF parameters

### Manual Verification

1. Generate a keystore:
   ```bash
   monoctl wallet generate --name test
   ```

2. Import into MetaMask:
   - Open MetaMask browser extension
   - Account > Import Account > JSON File
   - Select the keystore file
   - Enter password

3. Verify the address matches what `monoctl` displayed

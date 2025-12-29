package walletgen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"golang.org/x/crypto/sha3"
)

// Bech32 prefix for Monolythium addresses
const (
	Bech32PrefixAccAddr = "mono"
)

// secp256k1 curve parameters
var secp256k1Curve *secp256k1Params

func init() {
	secp256k1Curve = &secp256k1Params{}
	secp256k1Curve.P, _ = new(big.Int).SetString("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f", 16)
	secp256k1Curve.N, _ = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	secp256k1Curve.B, _ = new(big.Int).SetString("0000000000000000000000000000000000000000000000000000000000000007", 16)
	secp256k1Curve.Gx, _ = new(big.Int).SetString("79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798", 16)
	secp256k1Curve.Gy, _ = new(big.Int).SetString("483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8", 16)
	secp256k1Curve.BitSize = 256
	secp256k1Curve.Name = "secp256k1"
}

// secp256k1Params implements elliptic.Curve for secp256k1
type secp256k1Params struct {
	P       *big.Int
	N       *big.Int
	B       *big.Int
	Gx, Gy  *big.Int
	BitSize int
	Name    string
}

func (curve *secp256k1Params) Params() *elliptic.CurveParams {
	return &elliptic.CurveParams{
		P:       curve.P,
		N:       curve.N,
		B:       curve.B,
		Gx:      curve.Gx,
		Gy:      curve.Gy,
		BitSize: curve.BitSize,
		Name:    curve.Name,
	}
}

func (curve *secp256k1Params) IsOnCurve(x, y *big.Int) bool {
	// y^2 = x^3 + 7 (mod p)
	y2 := new(big.Int).Mul(y, y)
	y2.Mod(y2, curve.P)

	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)
	x3.Add(x3, curve.B)
	x3.Mod(x3, curve.P)

	return x3.Cmp(y2) == 0
}

func (curve *secp256k1Params) Add(x1, y1, x2, y2 *big.Int) (*big.Int, *big.Int) {
	z1 := zForAffine(x1, y1)
	z2 := zForAffine(x2, y2)
	return curve.affineFromJacobian(curve.addJacobian(x1, y1, z1, x2, y2, z2))
}

func (curve *secp256k1Params) Double(x1, y1 *big.Int) (*big.Int, *big.Int) {
	z1 := zForAffine(x1, y1)
	return curve.affineFromJacobian(curve.doubleJacobian(x1, y1, z1))
}

func (curve *secp256k1Params) ScalarMult(Bx, By *big.Int, k []byte) (*big.Int, *big.Int) {
	Bz := new(big.Int).SetInt64(1)
	x, y, z := new(big.Int), new(big.Int), new(big.Int)

	for _, byte := range k {
		for bitNum := 0; bitNum < 8; bitNum++ {
			x, y, z = curve.doubleJacobian(x, y, z)
			if byte&0x80 == 0x80 {
				x, y, z = curve.addJacobian(Bx, By, Bz, x, y, z)
			}
			byte <<= 1
		}
	}

	return curve.affineFromJacobian(x, y, z)
}

func (curve *secp256k1Params) ScalarBaseMult(k []byte) (*big.Int, *big.Int) {
	return curve.ScalarMult(curve.Gx, curve.Gy, k)
}

func zForAffine(x, y *big.Int) *big.Int {
	z := new(big.Int)
	if x.Sign() != 0 || y.Sign() != 0 {
		z.SetInt64(1)
	}
	return z
}

func (curve *secp256k1Params) affineFromJacobian(x, y, z *big.Int) (*big.Int, *big.Int) {
	if z.Sign() == 0 {
		return new(big.Int), new(big.Int)
	}

	zinv := new(big.Int).ModInverse(z, curve.P)
	zinvsq := new(big.Int).Mul(zinv, zinv)

	xOut := new(big.Int).Mul(x, zinvsq)
	xOut.Mod(xOut, curve.P)
	zinvsq.Mul(zinvsq, zinv)
	yOut := new(big.Int).Mul(y, zinvsq)
	yOut.Mod(yOut, curve.P)
	return xOut, yOut
}

func (curve *secp256k1Params) addJacobian(x1, y1, z1, x2, y2, z2 *big.Int) (*big.Int, *big.Int, *big.Int) {
	x3, y3, z3 := new(big.Int), new(big.Int), new(big.Int)
	if z1.Sign() == 0 {
		x3.Set(x2)
		y3.Set(y2)
		z3.Set(z2)
		return x3, y3, z3
	}
	if z2.Sign() == 0 {
		x3.Set(x1)
		y3.Set(y1)
		z3.Set(z1)
		return x3, y3, z3
	}

	z1z1 := new(big.Int).Mul(z1, z1)
	z1z1.Mod(z1z1, curve.P)
	z2z2 := new(big.Int).Mul(z2, z2)
	z2z2.Mod(z2z2, curve.P)

	u1 := new(big.Int).Mul(x1, z2z2)
	u1.Mod(u1, curve.P)
	u2 := new(big.Int).Mul(x2, z1z1)
	u2.Mod(u2, curve.P)
	h := new(big.Int).Sub(u2, u1)
	xEqual := h.Sign() == 0
	if h.Sign() == -1 {
		h.Add(h, curve.P)
	}
	i := new(big.Int).Lsh(h, 1)
	i.Mul(i, i)
	j := new(big.Int).Mul(h, i)

	s1 := new(big.Int).Mul(y1, z2)
	s1.Mul(s1, z2z2)
	s1.Mod(s1, curve.P)
	s2 := new(big.Int).Mul(y2, z1)
	s2.Mul(s2, z1z1)
	s2.Mod(s2, curve.P)
	r := new(big.Int).Sub(s2, s1)
	if r.Sign() == -1 {
		r.Add(r, curve.P)
	}
	yEqual := r.Sign() == 0
	if xEqual && yEqual {
		return curve.doubleJacobian(x1, y1, z1)
	}
	r.Lsh(r, 1)
	v := new(big.Int).Mul(u1, i)

	x3.Set(r)
	x3.Mul(x3, x3)
	x3.Sub(x3, j)
	x3.Sub(x3, v)
	x3.Sub(x3, v)
	x3.Mod(x3, curve.P)

	y3.Set(r)
	v.Sub(v, x3)
	y3.Mul(y3, v)
	s1.Mul(s1, j)
	s1.Lsh(s1, 1)
	y3.Sub(y3, s1)
	y3.Mod(y3, curve.P)

	z3.Add(z1, z2)
	z3.Mul(z3, z3)
	z3.Sub(z3, z1z1)
	z3.Sub(z3, z2z2)
	z3.Mul(z3, h)
	z3.Mod(z3, curve.P)

	return x3, y3, z3
}

func (curve *secp256k1Params) doubleJacobian(x, y, z *big.Int) (*big.Int, *big.Int, *big.Int) {
	a := new(big.Int).Mul(x, x)
	a.Mod(a, curve.P)
	b := new(big.Int).Mul(y, y)
	b.Mod(b, curve.P)
	c := new(big.Int).Mul(b, b)
	c.Mod(c, curve.P)

	d := new(big.Int).Add(x, b)
	d.Mul(d, d)
	d.Sub(d, a)
	d.Sub(d, c)
	d.Lsh(d, 1)
	d.Mod(d, curve.P)

	e := new(big.Int).Lsh(a, 1)
	e.Add(e, a)

	f := new(big.Int).Mul(e, e)

	x3 := new(big.Int).Lsh(d, 1)
	x3.Sub(f, x3)
	x3.Mod(x3, curve.P)

	y3 := new(big.Int).Sub(d, x3)
	y3.Mul(e, y3)
	c.Lsh(c, 3)
	y3.Sub(y3, c)
	y3.Mod(y3, curve.P)

	z3 := new(big.Int).Mul(y, z)
	z3.Lsh(z3, 1)
	z3.Mod(z3, curve.P)

	return x3, y3, z3
}

// Secp256k1 returns the secp256k1 elliptic curve
func Secp256k1() elliptic.Curve {
	return secp256k1Curve
}

// PrivateKeyToEVMAddress converts an ECDSA private key to an EVM address (0x...)
func PrivateKeyToEVMAddress(privKey *ecdsa.PrivateKey) string {
	pubBytes := pubKeyToUncompressed(&privKey.PublicKey)
	return PubKeyBytesToEVMAddress(pubBytes)
}

// PubKeyBytesToEVMAddress converts uncompressed public key bytes (65 bytes, 0x04 prefix) to EVM address
func PubKeyBytesToEVMAddress(pubBytes []byte) string {
	// Skip the 0x04 prefix if present
	if len(pubBytes) == 65 && pubBytes[0] == 0x04 {
		pubBytes = pubBytes[1:]
	}

	// Keccak256 hash of the public key
	hash := keccak256(pubBytes)

	// Take last 20 bytes
	addr := hash[len(hash)-20:]
	return "0x" + hex.EncodeToString(addr)
}

// PrivateKeyToBech32Address converts an ECDSA private key to a bech32 address
func PrivateKeyToBech32Address(privKey *ecdsa.PrivateKey, prefix string) (string, error) {
	evmAddr := PrivateKeyToEVMAddress(privKey)
	return EVMToBech32Address(evmAddr, prefix)
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

// pubKeyToUncompressed returns the uncompressed public key bytes (65 bytes with 0x04 prefix)
func pubKeyToUncompressed(pub *ecdsa.PublicKey) []byte {
	xBytes := pub.X.Bytes()
	yBytes := pub.Y.Bytes()

	// Ensure 32 bytes each
	x := make([]byte, 32)
	y := make([]byte, 32)
	copy(x[32-len(xBytes):], xBytes)
	copy(y[32-len(yBytes):], yBytes)

	result := make([]byte, 65)
	result[0] = 0x04
	copy(result[1:33], x)
	copy(result[33:65], y)
	return result
}

// keccak256 computes the Keccak256 hash
func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

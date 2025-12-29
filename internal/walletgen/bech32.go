package walletgen

import (
	"errors"
	"strings"
)

// Bech32 encoding charset
const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var charsetRev = [128]int8{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	15, -1, 10, 17, 21, 20, 26, 30, 7, 5, -1, -1, -1, -1, -1, -1,
	-1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1,
	1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1, -1,
	-1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1,
	1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1, -1,
}

// polymod calculates the bech32 checksum
func polymod(values []int) int {
	gen := []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := 1
	for _, v := range values {
		b := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ v
		for i := 0; i < 5; i++ {
			if (b>>i)&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

// hrpExpand expands the human-readable part for checksum
func hrpExpand(hrp string) []int {
	result := make([]int, 0, len(hrp)*2+1)
	for _, c := range hrp {
		result = append(result, int(c>>5))
	}
	result = append(result, 0)
	for _, c := range hrp {
		result = append(result, int(c&31))
	}
	return result
}

// verifyChecksum verifies the bech32 checksum
func verifyChecksum(hrp string, data []int) bool {
	values := append(hrpExpand(hrp), data...)
	return polymod(values) == 1
}

// createChecksum creates the bech32 checksum
func createChecksum(hrp string, data []int) []int {
	values := append(hrpExpand(hrp), data...)
	values = append(values, []int{0, 0, 0, 0, 0, 0}...)
	mod := polymod(values) ^ 1
	result := make([]int, 6)
	for i := 0; i < 6; i++ {
		result[i] = (mod >> (5 * (5 - i))) & 31
	}
	return result
}

// ConvertBits converts a byte slice from one bit-per-element to another
func ConvertBits(data []byte, fromBits, toBits int, pad bool) ([]byte, error) {
	acc := 0
	bits := 0
	ret := make([]byte, 0, len(data)*fromBits/toBits+1)
	maxv := (1 << toBits) - 1
	for _, value := range data {
		if int(value)>>fromBits != 0 {
			return nil, errors.New("invalid data range")
		}
		acc = (acc << fromBits) | int(value)
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			ret = append(ret, byte((acc>>bits)&maxv))
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits || ((acc<<(toBits-bits))&maxv) != 0 {
		return nil, errors.New("invalid padding")
	}
	return ret, nil
}

// Bech32Encode encodes a human-readable part and data bytes to a bech32 string
func Bech32Encode(hrp string, data []byte) (string, error) {
	// Convert 8-bit data to 5-bit
	conv, err := ConvertBits(data, 8, 5, true)
	if err != nil {
		return "", err
	}

	// Convert to int slice for checksum
	values := make([]int, len(conv))
	for i, v := range conv {
		values[i] = int(v)
	}

	// Create checksum
	checksum := createChecksum(hrp, values)

	// Build result
	combined := append(values, checksum...)
	var result strings.Builder
	result.WriteString(hrp)
	result.WriteString("1")
	for _, v := range combined {
		result.WriteByte(charset[v])
	}

	return result.String(), nil
}

// Bech32Decode decodes a bech32 string to human-readable part and data bytes
func Bech32Decode(bech string) (string, []byte, error) {
	// Find separator
	pos := strings.LastIndex(bech, "1")
	if pos < 1 || pos+7 > len(bech) || len(bech) > 90 {
		return "", nil, errors.New("invalid bech32 string")
	}

	// Check for mixed case
	if bech != strings.ToLower(bech) && bech != strings.ToUpper(bech) {
		return "", nil, errors.New("mixed case in bech32 string")
	}
	bech = strings.ToLower(bech)

	hrp := bech[:pos]
	dataStr := bech[pos+1:]

	// Decode data part
	data := make([]int, len(dataStr))
	for i, c := range dataStr {
		if c < 0 || c > 127 || charsetRev[c] == -1 {
			return "", nil, errors.New("invalid character in bech32 string")
		}
		data[i] = int(charsetRev[c])
	}

	// Verify checksum
	if !verifyChecksum(hrp, data) {
		return "", nil, errors.New("invalid checksum")
	}

	// Remove checksum
	data = data[:len(data)-6]

	// Convert 5-bit to 8-bit
	conv := make([]byte, len(data))
	for i, v := range data {
		conv[i] = byte(v)
	}
	result, err := ConvertBits(conv, 5, 8, false)
	if err != nil {
		return "", nil, err
	}

	return hrp, result, nil
}

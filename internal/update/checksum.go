package update

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// ChecksumEntry represents a single entry in a checksums file.
type ChecksumEntry struct {
	Hash     string
	Filename string
}

// ParseChecksums parses a checksums file in common formats:
// - "hash  filename" (sha256sum output)
// - "hash *filename" (binary mode)
// - "filename: hash" (alternative format)
func ParseChecksums(data []byte) ([]ChecksumEntry, error) {
	var entries []ChecksumEntry
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entry, err := parseChecksumLine(line)
		if err != nil {
			continue // Skip invalid lines
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse checksums: %w", err)
	}

	return entries, nil
}

// parseChecksumLine parses a single checksum line.
func parseChecksumLine(line string) (ChecksumEntry, error) {
	// Format: "hash  filename" or "hash *filename"
	if len(line) > 64 && (line[64] == ' ' || line[64] == '*') {
		hash := line[:64]
		filename := strings.TrimPrefix(line[64:], " ")
		filename = strings.TrimPrefix(filename, "*")
		filename = strings.TrimSpace(filename)
		return ChecksumEntry{Hash: hash, Filename: filename}, nil
	}

	// Format: "filename: hash"
	if idx := strings.Index(line, ":"); idx != -1 {
		filename := strings.TrimSpace(line[:idx])
		hash := strings.TrimSpace(line[idx+1:])
		if len(hash) == 64 {
			return ChecksumEntry{Hash: hash, Filename: filename}, nil
		}
	}

	// Try splitting by whitespace
	parts := strings.Fields(line)
	if len(parts) == 2 {
		// Determine which is hash (64 hex chars)
		if len(parts[0]) == 64 && isHexString(parts[0]) {
			return ChecksumEntry{Hash: parts[0], Filename: parts[1]}, nil
		}
		if len(parts[1]) == 64 && isHexString(parts[1]) {
			return ChecksumEntry{Hash: parts[1], Filename: parts[0]}, nil
		}
	}

	return ChecksumEntry{}, fmt.Errorf("invalid checksum line: %s", line)
}

// isHexString returns true if s is a valid hex string.
func isHexString(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// FindChecksum finds the checksum for a given filename in the entries.
func FindChecksum(entries []ChecksumEntry, filename string) (string, bool) {
	for _, entry := range entries {
		// Match exact filename or just the base name
		if entry.Filename == filename ||
			strings.HasSuffix(entry.Filename, "/"+filename) ||
			strings.TrimPrefix(entry.Filename, "./") == filename {
			return entry.Hash, true
		}
	}
	return "", false
}

// ComputeFileSHA256 computes the SHA256 hash of a file.
func ComputeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeDataSHA256 computes the SHA256 hash of data.
func ComputeDataSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// VerifyChecksum verifies that a file matches the expected checksum.
func VerifyChecksum(path, expected string) error {
	actual, err := ComputeFileSHA256(path)
	if err != nil {
		return err
	}

	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

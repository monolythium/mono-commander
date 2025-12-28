package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []ChecksumEntry
		wantErr bool
	}{
		{
			name:  "sha256sum format",
			input: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1  monoctl_linux_amd64\n",
			want: []ChecksumEntry{
				{Hash: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1", Filename: "monoctl_linux_amd64"},
			},
		},
		{
			name:  "binary mode format",
			input: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1 *monoctl_darwin_arm64\n",
			want: []ChecksumEntry{
				{Hash: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1", Filename: "monoctl_darwin_arm64"},
			},
		},
		{
			name:  "multiple entries",
			input: "aaa123def456abc123def456abc123def456abc123def456abc123def456abc1  monoctl_linux_amd64\nbbb123def456abc123def456abc123def456abc123def456abc123def456abc1  monoctl_darwin_arm64\n",
			want: []ChecksumEntry{
				{Hash: "aaa123def456abc123def456abc123def456abc123def456abc123def456abc1", Filename: "monoctl_linux_amd64"},
				{Hash: "bbb123def456abc123def456abc123def456abc123def456abc123def456abc1", Filename: "monoctl_darwin_arm64"},
			},
		},
		{
			name:  "with comments",
			input: "# SHA256 checksums\nabc123def456abc123def456abc123def456abc123def456abc123def456abc1  monoctl_linux_amd64\n",
			want: []ChecksumEntry{
				{Hash: "abc123def456abc123def456abc123def456abc123def456abc123def456abc1", Filename: "monoctl_linux_amd64"},
			},
		},
		{
			name:    "empty input",
			input:   "",
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChecksums([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseChecksums() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ParseChecksums() got %d entries, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].Hash != tt.want[i].Hash {
					t.Errorf("entry[%d].Hash = %q, want %q", i, got[i].Hash, tt.want[i].Hash)
				}
				if got[i].Filename != tt.want[i].Filename {
					t.Errorf("entry[%d].Filename = %q, want %q", i, got[i].Filename, tt.want[i].Filename)
				}
			}
		})
	}
}

func TestFindChecksum(t *testing.T) {
	entries := []ChecksumEntry{
		{Hash: "hash1", Filename: "monoctl_linux_amd64"},
		{Hash: "hash2", Filename: "monoctl_darwin_arm64"},
		{Hash: "hash3", Filename: "./monoctl_windows_amd64.exe"},
	}

	tests := []struct {
		filename string
		wantHash string
		wantOK   bool
	}{
		{"monoctl_linux_amd64", "hash1", true},
		{"monoctl_darwin_arm64", "hash2", true},
		{"monoctl_windows_amd64.exe", "hash3", true},
		{"monoctl_darwin_amd64", "", false},
		{"nonexistent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			hash, ok := FindChecksum(entries, tt.filename)
			if ok != tt.wantOK {
				t.Errorf("FindChecksum(%q) ok = %v, want %v", tt.filename, ok, tt.wantOK)
			}
			if hash != tt.wantHash {
				t.Errorf("FindChecksum(%q) hash = %q, want %q", tt.filename, hash, tt.wantHash)
			}
		})
	}
}

func TestComputeFileSHA256(t *testing.T) {
	// Create a temp file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile")
	content := []byte("hello world\n")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	hash, err := ComputeFileSHA256(testFile)
	if err != nil {
		t.Fatalf("ComputeFileSHA256() error = %v", err)
	}

	// Verify it returns a 64-char hex string
	if len(hash) != 64 {
		t.Errorf("ComputeFileSHA256() returned hash of length %d, want 64", len(hash))
	}

	// Test nonexistent file
	_, err = ComputeFileSHA256("/nonexistent/file")
	if err == nil {
		t.Error("ComputeFileSHA256() should error on nonexistent file")
	}
}

func TestComputeDataSHA256(t *testing.T) {
	data := []byte("test data")
	hash := ComputeDataSHA256(data)

	if len(hash) != 64 {
		t.Errorf("ComputeDataSHA256() returned hash of length %d, want 64", len(hash))
	}

	// Same data should produce same hash
	hash2 := ComputeDataSHA256(data)
	if hash != hash2 {
		t.Error("ComputeDataSHA256() should be deterministic")
	}

	// Different data should produce different hash
	hash3 := ComputeDataSHA256([]byte("different data"))
	if hash == hash3 {
		t.Error("ComputeDataSHA256() should produce different hashes for different data")
	}
}

func TestVerifyChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile")
	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Get the actual hash
	actualHash, _ := ComputeFileSHA256(testFile)

	// Should succeed with correct hash
	err := VerifyChecksum(testFile, actualHash)
	if err != nil {
		t.Errorf("VerifyChecksum() with correct hash: %v", err)
	}

	// Should fail with wrong hash
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	err = VerifyChecksum(testFile, wrongHash)
	if err == nil {
		t.Error("VerifyChecksum() should fail with wrong hash")
	}
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abc123", true},
		{"ABC123", true},
		{"abcdef0123456789", true},
		{"xyz", false},
		{"abc 123", false},
		{"", true}, // empty is valid hex
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isHexString(tt.input)
			if got != tt.want {
				t.Errorf("isHexString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

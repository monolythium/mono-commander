package monod

import (
	"runtime"
	"strings"
	"testing"
)

func TestDetectOSArch(t *testing.T) {
	result, err := detectOSArch()
	if err != nil {
		t.Fatalf("detectOSArch() failed: %v", err)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if !strings.Contains(result, goos) {
		t.Errorf("detectOSArch() = %q, should contain OS %q", result, goos)
	}

	if !strings.Contains(result, goarch) {
		t.Errorf("detectOSArch() = %q, should contain arch %q", result, goarch)
	}
}

func TestBinaryInstallPath(t *testing.T) {
	userPath := BinaryInstallPath(false)
	if !strings.HasSuffix(userPath, "monod") {
		t.Errorf("BinaryInstallPath(false) should end with 'monod', got %s", userPath)
	}

	systemPath := BinaryInstallPath(true)
	if systemPath != "/usr/local/bin/monod" {
		t.Errorf("BinaryInstallPath(true) = %s, want /usr/local/bin/monod", systemPath)
	}
}

func TestInstallDryRun(t *testing.T) {
	opts := InstallOptions{
		DryRun: true,
	}

	result := Install(opts)

	if !result.Success {
		t.Errorf("Dry run should succeed, got error: %v", result.Error)
	}

	if result.Downloaded {
		t.Error("Dry run should not download anything")
	}
}

func TestFetchChecksum(t *testing.T) {
	checksumURL := "https://github.com/monolythium/mono-core/releases/download/v0.1.0/checksums.txt"

	checksum, err := fetchChecksum(checksumURL, "linux-amd64")
	if err != nil {
		t.Fatalf("fetchChecksum() failed: %v", err)
	}

	if checksum == "" {
		t.Error("fetchChecksum() returned empty checksum")
	}

	if len(checksum) != 64 {
		t.Errorf("fetchChecksum() returned checksum of length %d, want 64", len(checksum))
	}

	expectedChecksum := "c9a2918ea7c52f9e91e4fadcd02814092fa1349651623afa034fb961582e91e1"
	if checksum != expectedChecksum {
		t.Errorf("fetchChecksum() = %q, want %q", checksum, expectedChecksum)
	}
}

func TestFetchChecksumDarwinArm64(t *testing.T) {
	checksumURL := "https://github.com/monolythium/mono-core/releases/download/v0.1.0/checksums.txt"

	checksum, err := fetchChecksum(checksumURL, "darwin-arm64")
	if err != nil {
		t.Fatalf("fetchChecksum() failed: %v", err)
	}

	expectedChecksum := "0c888607d234b6832e73ed90e7e95330fb0173c65dc15fdf0c16dbab08b4a7ad"
	if checksum != expectedChecksum {
		t.Errorf("fetchChecksum() = %q, want %q", checksum, expectedChecksum)
	}
}

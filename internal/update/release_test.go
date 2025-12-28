package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFetchLatestRelease(t *testing.T) {
	// Create mock release response
	mockRelease := ReleaseInfo{
		TagName: "v1.2.3",
		Name:    "Release v1.2.3",
		Assets: []Asset{
			{
				Name:               "monoctl_darwin_arm64",
				BrowserDownloadURL: "https://example.com/monoctl_darwin_arm64",
				Size:               1234567,
			},
			{
				Name:               "monoctl_linux_amd64",
				BrowserDownloadURL: "https://example.com/monoctl_linux_amd64",
				Size:               2345678,
			},
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: "https://example.com/checksums.txt",
				Size:               256,
			},
		},
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/test/repo/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mockRelease)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client with mock server
	client := &Client{
		HTTPClient: server.Client(),
		RepoOwner:  "test",
		RepoName:   "repo",
		APIURL:     server.URL,
	}

	release, err := client.FetchLatestRelease()
	if err != nil {
		t.Fatalf("FetchLatestRelease() error = %v", err)
	}

	if release.TagName != "v1.2.3" {
		t.Errorf("FetchLatestRelease() TagName = %q, want %q", release.TagName, "v1.2.3")
	}

	if len(release.Assets) != 3 {
		t.Errorf("FetchLatestRelease() got %d assets, want 3", len(release.Assets))
	}
}

func TestFetchLatestReleaseNotFound(t *testing.T) {
	// Create mock server returning 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		RepoOwner:  "test",
		RepoName:   "repo",
		APIURL:     server.URL,
	}

	_, err := client.FetchLatestRelease()
	if err == nil {
		t.Error("FetchLatestRelease() should error on 404")
	}
}

func TestCheck(t *testing.T) {
	mockRelease := ReleaseInfo{
		TagName: "v2.0.0",
		Name:    "Release v2.0.0",
		Assets: []Asset{
			{
				Name:               "monoctl_" + runtime.GOOS + "_" + runtime.GOARCH,
				BrowserDownloadURL: "https://example.com/monoctl",
				Size:               1234567,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockRelease)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		RepoOwner:  "test",
		RepoName:   "repo",
		APIURL:     server.URL,
	}

	result, err := client.Check("v1.0.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if !result.UpdateAvailable {
		t.Error("Check() UpdateAvailable = false, want true")
	}

	if result.LatestVersion != "v2.0.0" {
		t.Errorf("Check() LatestVersion = %q, want %q", result.LatestVersion, "v2.0.0")
	}

	if result.DownloadURL == "" {
		t.Error("Check() DownloadURL should not be empty")
	}
}

func TestCheckNoUpdate(t *testing.T) {
	mockRelease := ReleaseInfo{
		TagName: "v1.0.0",
		Name:    "Release v1.0.0",
		Assets:  []Asset{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockRelease)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		RepoOwner:  "test",
		RepoName:   "repo",
		APIURL:     server.URL,
	}

	result, err := client.Check("v1.0.0")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.UpdateAvailable {
		t.Error("Check() UpdateAvailable = true, want false (same version)")
	}
}

func TestCheckDevVersion(t *testing.T) {
	mockRelease := ReleaseInfo{
		TagName: "v1.0.0",
		Name:    "Release v1.0.0",
		Assets:  []Asset{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockRelease)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		RepoOwner:  "test",
		RepoName:   "repo",
		APIURL:     server.URL,
	}

	// Dev versions should show update available
	result, err := client.Check("dev")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if !result.UpdateAvailable {
		t.Error("Check() with dev version should show update available")
	}
}

func TestFindMatchingAsset(t *testing.T) {
	assets := []Asset{
		{Name: "monoctl_darwin_arm64", BrowserDownloadURL: "https://example.com/1"},
		{Name: "monoctl_darwin_amd64", BrowserDownloadURL: "https://example.com/2"},
		{Name: "monoctl_linux_amd64", BrowserDownloadURL: "https://example.com/3"},
		{Name: "monoctl_linux_arm64", BrowserDownloadURL: "https://example.com/4"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
	}

	client := NewClient()

	tests := []struct {
		os, arch string
		wantName string
	}{
		{"darwin", "arm64", "monoctl_darwin_arm64"},
		{"darwin", "amd64", "monoctl_darwin_amd64"},
		{"linux", "amd64", "monoctl_linux_amd64"},
		{"linux", "arm64", "monoctl_linux_arm64"},
	}

	for _, tt := range tests {
		t.Run(tt.os+"_"+tt.arch, func(t *testing.T) {
			asset := client.FindMatchingAsset(assets, tt.os, tt.arch)
			if asset == nil {
				t.Fatalf("FindMatchingAsset(%q, %q) returned nil", tt.os, tt.arch)
			}
			if asset.Name != tt.wantName {
				t.Errorf("FindMatchingAsset(%q, %q) = %q, want %q", tt.os, tt.arch, asset.Name, tt.wantName)
			}
		})
	}
}

func TestFindMatchingAssetNotFound(t *testing.T) {
	assets := []Asset{
		{Name: "monoctl_darwin_arm64", BrowserDownloadURL: "https://example.com/1"},
	}

	client := NewClient()
	asset := client.FindMatchingAsset(assets, "windows", "amd64")
	if asset != nil {
		t.Error("FindMatchingAsset() should return nil for unsupported OS")
	}
}

func TestDownloadAsset(t *testing.T) {
	binaryContent := []byte("mock binary content")
	checksumContent := "abc123def456abc123def456abc123def456abc123def456abc123def456abc1  monoctl_test\n"

	// Calculate actual checksum of mock binary
	actualChecksum := ComputeDataSHA256(binaryContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/monoctl_test":
			w.Write(binaryContent)
		case "/checksums.txt":
			// Use actual checksum so verification passes
			w.Write([]byte(actualChecksum + "  monoctl_test\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "monoctl_test")

	client := &Client{
		HTTPClient: server.Client(),
		RepoOwner:  "test",
		RepoName:   "repo",
		APIURL:     server.URL,
	}

	asset := &Asset{
		Name:               "monoctl_test",
		BrowserDownloadURL: server.URL + "/monoctl_test",
	}

	checksumAsset := &Asset{
		Name:               "checksums.txt",
		BrowserDownloadURL: server.URL + "/checksums.txt",
	}

	err := client.DownloadAsset(asset, destPath)
	if err != nil {
		t.Fatalf("DownloadAsset() error = %v", err)
	}

	// Verify file was downloaded
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}
	if string(content) != string(binaryContent) {
		t.Errorf("Downloaded content = %q, want %q", content, binaryContent)
	}

	// Now test with checksum verification
	destPath2 := filepath.Join(tmpDir, "monoctl_test2")
	err = client.DownloadAndVerify(asset, checksumAsset, destPath2)
	if err != nil {
		t.Fatalf("DownloadAndVerify() error = %v", err)
	}

	content2, err := os.ReadFile(destPath2)
	if err != nil {
		t.Fatalf("Failed to read verified file: %v", err)
	}
	if string(content2) != string(binaryContent) {
		t.Errorf("Verified content = %q, want %q", content2, binaryContent)
	}

	// Checksum file should not exist since it's removed after verification
	_ = checksumContent // Mark as used
}

func TestDownloadAndVerifyBadChecksum(t *testing.T) {
	binaryContent := []byte("mock binary content")
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000  monoctl_test\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/monoctl_test":
			w.Write(binaryContent)
		case "/checksums.txt":
			w.Write([]byte(wrongChecksum))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "monoctl_test")

	client := &Client{
		HTTPClient: server.Client(),
		RepoOwner:  "test",
		RepoName:   "repo",
		APIURL:     server.URL,
	}

	asset := &Asset{
		Name:               "monoctl_test",
		BrowserDownloadURL: server.URL + "/monoctl_test",
	}

	checksumAsset := &Asset{
		Name:               "checksums.txt",
		BrowserDownloadURL: server.URL + "/checksums.txt",
	}

	err := client.DownloadAndVerify(asset, checksumAsset, destPath)
	if err == nil {
		t.Error("DownloadAndVerify() should fail with wrong checksum")
	}
}

func TestFindChecksumAsset(t *testing.T) {
	assets := []Asset{
		{Name: "monoctl_darwin_arm64", BrowserDownloadURL: "https://example.com/1"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
	}

	client := NewClient()
	checksumAsset := client.FindChecksumAsset(assets)
	if checksumAsset == nil {
		t.Fatal("FindChecksumAsset() returned nil")
	}
	if checksumAsset.Name != "checksums.txt" {
		t.Errorf("FindChecksumAsset() = %q, want %q", checksumAsset.Name, "checksums.txt")
	}
}

func TestFindChecksumAssetVariants(t *testing.T) {
	tests := []struct {
		name       string
		assetNames []string
		wantName   string
	}{
		{
			name:       "checksums.txt",
			assetNames: []string{"monoctl", "checksums.txt"},
			wantName:   "checksums.txt",
		},
		{
			name:       "SHA256SUMS",
			assetNames: []string{"monoctl", "SHA256SUMS"},
			wantName:   "SHA256SUMS",
		},
		{
			name:       "sha256sums.txt",
			assetNames: []string{"monoctl", "sha256sums.txt"},
			wantName:   "sha256sums.txt",
		},
		{
			name:       "not found",
			assetNames: []string{"monoctl", "readme.txt"},
			wantName:   "",
		},
	}

	client := NewClient()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var assets []Asset
			for _, name := range tt.assetNames {
				assets = append(assets, Asset{Name: name, BrowserDownloadURL: "https://example.com/" + name})
			}

			checksumAsset := client.FindChecksumAsset(assets)
			if tt.wantName == "" {
				if checksumAsset != nil {
					t.Errorf("FindChecksumAsset() = %q, want nil", checksumAsset.Name)
				}
			} else {
				if checksumAsset == nil {
					t.Fatal("FindChecksumAsset() returned nil")
				}
				if checksumAsset.Name != tt.wantName {
					t.Errorf("FindChecksumAsset() = %q, want %q", checksumAsset.Name, tt.wantName)
				}
			}
		})
	}
}

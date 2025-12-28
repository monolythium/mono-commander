package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	// DefaultRepoOwner is the default GitHub org/user.
	DefaultRepoOwner = "monolythium"
	// DefaultRepoName is the default repository name.
	DefaultRepoName = "mono-commander"
	// GitHubAPIURL is the base URL for GitHub API.
	GitHubAPIURL = "https://api.github.com"
)

// ReleaseInfo contains information about a GitHub release.
type ReleaseInfo struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// CheckResult contains the result of an update check.
type CheckResult struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	PublishedAt     time.Time `json:"published_at"`
	UpdateAvailable bool      `json:"update_available"`
	ReleaseURL      string    `json:"release_url"`
	DownloadURL     string    `json:"download_url,omitempty"`
	ChecksumURL     string    `json:"checksum_url,omitempty"`
	AssetName       string    `json:"asset_name,omitempty"`
	Status          string    `json:"status"` // "up-to-date", "update-available", "unknown"
	Error           string    `json:"error,omitempty"`
}

// Client is the update client for checking and applying updates.
type Client struct {
	HTTPClient *http.Client
	RepoOwner  string
	RepoName   string
	APIURL     string
}

// NewClient creates a new update client with defaults.
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		RepoOwner:  DefaultRepoOwner,
		RepoName:   DefaultRepoName,
		APIURL:     GitHubAPIURL,
	}
}

// FetchLatestRelease fetches the latest release from GitHub.
func (c *Client) FetchLatestRelease() (*ReleaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.APIURL, c.RepoOwner, c.RepoName)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "mono-commander")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found for %s/%s", c.RepoOwner, c.RepoName)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (%d): %s", resp.StatusCode, string(body))
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// Check checks for updates against the current version.
func (c *Client) Check(currentVersion string) (*CheckResult, error) {
	result := &CheckResult{
		CurrentVersion: currentVersion,
		Status:         "unknown",
	}

	release, err := c.FetchLatestRelease()
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}

	result.LatestVersion = release.TagName
	result.PublishedAt = release.PublishedAt
	result.ReleaseURL = release.HTMLURL

	// Parse versions
	current, err := ParseVersion(currentVersion)
	if err != nil || current.IsDev() {
		// Can't compare dev versions, suggest update if there's a release
		result.UpdateAvailable = true
		result.Status = "update-available"
	} else {
		latest, err := ParseVersion(release.TagName)
		if err != nil {
			result.Error = fmt.Sprintf("failed to parse latest version: %v", err)
			return result, nil
		}

		if current.LessThan(latest) {
			result.UpdateAvailable = true
			result.Status = "update-available"
		} else {
			result.Status = "up-to-date"
		}
	}

	// Find matching asset
	asset := c.FindMatchingAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if asset != nil {
		result.DownloadURL = asset.BrowserDownloadURL
		result.AssetName = asset.Name
	}

	// Find checksum file
	checksumAsset := c.FindChecksumAsset(release.Assets)
	if checksumAsset != nil {
		result.ChecksumURL = checksumAsset.BrowserDownloadURL
	}

	return result, nil
}

// FindMatchingAsset finds a release asset matching the current OS/arch.
func (c *Client) FindMatchingAsset(assets []Asset, os, arch string) *Asset {
	pattern := AssetNamePattern(os, arch)

	for i, asset := range assets {
		// Skip checksum files and archives we don't support
		if isChecksumFile(asset.Name) {
			continue
		}

		if pattern.MatchString(asset.Name) {
			return &assets[i]
		}
	}

	return nil
}

// FindChecksumAsset finds the checksum file in the release assets.
func (c *Client) FindChecksumAsset(assets []Asset) *Asset {
	for i, asset := range assets {
		if isChecksumFile(asset.Name) {
			return &assets[i]
		}
	}
	return nil
}

// isChecksumFile returns true if the asset name looks like a checksum file.
func isChecksumFile(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "checksum") ||
		strings.Contains(name, "sha256") ||
		strings.HasSuffix(name, ".sha256") ||
		strings.HasSuffix(name, ".sha256sum") ||
		name == "checksums.txt"
}

// ListAvailableAssets returns a list of available assets for display.
func ListAvailableAssets(assets []Asset) []string {
	var names []string
	for _, asset := range assets {
		if !isChecksumFile(asset.Name) {
			names = append(names, asset.Name)
		}
	}
	return names
}

// DownloadAsset downloads an asset to the specified destination path.
func (c *Client) DownloadAsset(asset *Asset, destPath string) error {
	req, err := http.NewRequest(http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "mono-commander")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination file
	f, err := createFile(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// DownloadAndVerify downloads an asset and verifies its checksum.
func (c *Client) DownloadAndVerify(asset, checksumAsset *Asset, destPath string) error {
	// First download the checksums file
	checksumData, err := c.fetchChecksumData(checksumAsset)
	if err != nil {
		return fmt.Errorf("failed to fetch checksums: %w", err)
	}

	// Parse checksums
	entries, err := ParseChecksums(checksumData)
	if err != nil {
		return fmt.Errorf("failed to parse checksums: %w", err)
	}

	// Find the expected checksum for this asset
	expectedHash, found := FindChecksum(entries, asset.Name)
	if !found {
		return fmt.Errorf("checksum not found for asset %q", asset.Name)
	}

	// Download the asset
	if err := c.DownloadAsset(asset, destPath); err != nil {
		return err
	}

	// Verify the checksum
	if err := VerifyChecksum(destPath, expectedHash); err != nil {
		// Remove the downloaded file if verification fails
		removeFile(destPath)
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	return nil
}

// fetchChecksumData fetches and returns the raw checksum file content.
func (c *Client) fetchChecksumData(checksumAsset *Asset) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, checksumAsset.BrowserDownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "mono-commander")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch checksum: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksum fetch failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// createFile creates a new file for writing.
func createFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
}

// removeFile removes a file if it exists.
func removeFile(path string) {
	os.Remove(path)
}

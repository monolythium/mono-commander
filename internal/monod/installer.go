package monod

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	DefaultReleaseURL = "https://github.com/monolythium/mono-core/releases"
	DefaultVersion    = "v1.1.2"
)

type InstallOptions struct {
	URL           string
	SHA256        string
	Version       string
	UseSystemPath bool
	Insecure      bool
	DryRun        bool
}

type InstallResult struct {
	Success     bool
	InstallPath string
	Version     string
	SHA256      string
	Downloaded  bool
	Error       error
	Steps       []InstallStep
}

type InstallStep struct {
	Name    string
	Status  string
	Message string
}

func Install(opts InstallOptions) *InstallResult {
	result := &InstallResult{
		Steps: make([]InstallStep, 0),
	}

	result.Steps = append(result.Steps, InstallStep{Name: "Validate options", Status: "pending"})

	if opts.Version == "" {
		opts.Version = DefaultVersion
	}

	osArch, err := detectOSArch()
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result
	}

	if opts.URL == "" {
		opts.URL = fmt.Sprintf("%s/download/%s/monod-%s", DefaultReleaseURL, opts.Version, osArch)
	}

	checksumURL := fmt.Sprintf("%s/download/%s/checksums.txt", DefaultReleaseURL, opts.Version)

	if opts.SHA256 == "" && !opts.Insecure {
		fetchedChecksum, err := fetchChecksum(checksumURL, osArch)
		if err != nil {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Failed to fetch checksum: %v", err)
			result.Error = fmt.Errorf("failed to fetch checksum from %s: %w. Use --sha256 <hash> or --insecure to skip", checksumURL, err)
			return result
		}
		opts.SHA256 = fetchedChecksum
	}

	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Target: %s", osArch)

	result.Steps = append(result.Steps, InstallStep{Name: "Determine install path", Status: "pending"})
	installPath := BinaryInstallPath(opts.UseSystemPath)
	result.InstallPath = installPath
	result.Version = opts.Version

	if opts.DryRun {
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Would install to: %s", installPath)

		result.Steps = append(result.Steps, InstallStep{
			Name:    "Download binary",
			Status:  "skipped",
			Message: fmt.Sprintf("Would download from: %s", opts.URL),
		})

		if opts.SHA256 != "" {
			result.Steps = append(result.Steps, InstallStep{
				Name:    "Verify checksum",
				Status:  "skipped",
				Message: fmt.Sprintf("Would verify SHA256: %s", opts.SHA256),
			})
		}

		result.Steps = append(result.Steps, InstallStep{
			Name:    "Install binary",
			Status:  "skipped",
			Message: fmt.Sprintf("Would make executable: %s", installPath),
		})

		result.Success = true
		return result
	}

	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = installPath

	result.Steps = append(result.Steps, InstallStep{Name: "Create install directory", Status: "pending"})
	installDir := filepath.Dir(installPath)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = fmt.Errorf("failed to create install directory: %w", err)
		return result
	}
	result.Steps[len(result.Steps)-1].Status = "success"

	result.Steps = append(result.Steps, InstallStep{Name: "Download binary", Status: "pending"})

	tmpFile, err := downloadToTemp(opts.URL)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err
		return result
	}
	defer os.Remove(tmpFile)
	result.Downloaded = true
	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = "Downloaded to temp file"

	if opts.SHA256 != "" {
		result.Steps = append(result.Steps, InstallStep{Name: "Verify checksum", Status: "pending"})

		actualHash, err := computeSHA256(tmpFile)
		if err != nil {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = err.Error()
			result.Error = err
			return result
		}

		if actualHash != opts.SHA256 {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("expected %s, got %s", opts.SHA256, actualHash)
			result.Error = fmt.Errorf("checksum mismatch: expected %s, got %s", opts.SHA256, actualHash)
			return result
		}

		result.SHA256 = actualHash
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = "Checksum verified"
	} else {
		result.Steps = append(result.Steps, InstallStep{
			Name:    "Verify checksum",
			Status:  "skipped",
			Message: "Insecure mode - skipping verification",
		})
	}

	result.Steps = append(result.Steps, InstallStep{Name: "Install binary", Status: "pending"})

	if err := copyFile(tmpFile, installPath); err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = fmt.Errorf("failed to install binary: %w", err)
		return result
	}

	if err := os.Chmod(installPath, 0755); err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = fmt.Errorf("failed to make binary executable: %w", err)
		return result
	}

	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Installed to: %s", installPath)
	result.Success = true

	return result
}

func BinaryInstallPath(useSystemPath bool) string {
	if useSystemPath {
		return "/usr/local/bin/monod"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin", "monod")
}

func detectOSArch() (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var osStr, archStr string

	switch goos {
	case "linux":
		osStr = "linux"
	case "darwin":
		osStr = "darwin"
	default:
		return "", fmt.Errorf("unsupported OS: %s (supported: linux, darwin)", goos)
	}

	switch goarch {
	case "amd64":
		archStr = "amd64"
	case "arm64":
		archStr = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture: %s (supported: amd64, arm64)", goarch)
	}

	return fmt.Sprintf("%s-%s", osStr, archStr), nil
}

func fetchChecksum(checksumURL, osArch string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(checksumURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch checksums: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read checksums: %w", err)
	}

	lines := strings.Split(string(body), "\n")
	target := fmt.Sprintf("monod-%s", osArch)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		checksum := parts[0]
		filename := parts[1]

		if filename == target {
			return checksum, nil
		}
	}

	return "", fmt.Errorf("checksum not found for %s in checksums.txt", target)
}

func downloadToTemp(url string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "monod-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

func computeSHA256(path string) (string, error) {
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

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func BinaryExists(useSystemPath bool) bool {
	path := BinaryInstallPath(useSystemPath)
	_, err := os.Stat(path)
	return err == nil
}

func GetInstalledVersion(useSystemPath bool) (string, error) {
	path := BinaryInstallPath(useSystemPath)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("binary not installed at %s", path)
	}

	return "installed", nil
}

func Uninstall(useSystemPath, dryRun bool) error {
	path := BinaryInstallPath(useSystemPath)

	if dryRun {
		return nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(path)
}

// Package mesh provides Mesh/Rosetta API sidecar management for mono-commander.
package mesh

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// InstallOptions contains options for installing the mesh binary.
type InstallOptions struct {
	// URL is the download URL for the binary.
	URL string

	// SHA256 is the expected SHA256 checksum of the binary.
	SHA256 string

	// Version is the version to install.
	Version string

	// UseSystemPath installs to /usr/local/bin instead of ~/.local/bin.
	UseSystemPath bool

	// Insecure allows installation without checksum verification.
	Insecure bool

	// DryRun shows what would be done without making changes.
	DryRun bool
}

// InstallResult contains the result of an installation.
type InstallResult struct {
	// Success indicates if the installation was successful.
	Success bool

	// InstallPath is the path where the binary was installed.
	InstallPath string

	// Version is the installed version.
	Version string

	// SHA256 is the checksum of the installed binary.
	SHA256 string

	// Downloaded indicates if a download was performed.
	Downloaded bool

	// Error contains any error message.
	Error error

	// Steps contains the installation steps.
	Steps []InstallStep
}

// InstallStep represents a step in the installation process.
type InstallStep struct {
	Name    string
	Status  string // "pending", "success", "failed", "skipped"
	Message string
}

// Install downloads and installs the mesh binary.
func Install(opts InstallOptions) *InstallResult {
	result := &InstallResult{
		Steps: make([]InstallStep, 0),
	}

	// Step 1: Validate options
	result.Steps = append(result.Steps, InstallStep{Name: "Validate options", Status: "pending"})

	if opts.URL == "" {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = "No download URL provided"
		result.Error = fmt.Errorf("no download URL provided. Please specify --url or configure a default download source")
		return result
	}

	if opts.SHA256 == "" && !opts.Insecure {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = "Checksum required"
		result.Error = fmt.Errorf("SHA256 checksum required for security. Use --sha256 <hash> or --insecure to skip (not recommended)")
		return result
	}

	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 2: Determine install path
	result.Steps = append(result.Steps, InstallStep{Name: "Determine install path", Status: "pending"})
	installPath := BinaryInstallPath(opts.UseSystemPath)
	result.InstallPath = installPath
	result.Version = opts.Version

	if opts.DryRun {
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Would install to: %s", installPath)

		// Add dry-run steps
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

	// Step 3: Create install directory
	result.Steps = append(result.Steps, InstallStep{Name: "Create install directory", Status: "pending"})
	installDir := filepath.Dir(installPath)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = fmt.Errorf("failed to create install directory: %w", err)
		return result
	}
	result.Steps[len(result.Steps)-1].Status = "success"

	// Step 4: Download binary
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

	// Step 5: Verify checksum
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

	// Step 6: Install binary
	result.Steps = append(result.Steps, InstallStep{Name: "Install binary", Status: "pending"})

	// Copy to install path
	if err := copyFile(tmpFile, installPath); err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = fmt.Errorf("failed to install binary: %w", err)
		return result
	}

	// Make executable
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

// downloadToTemp downloads a URL to a temporary file.
func downloadToTemp(url string) (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "mono-mesh-rosetta-*")
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

// computeSHA256 computes the SHA256 checksum of a file.
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

// copyFile copies a file from src to dst.
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

// Uninstall removes the mesh binary.
func Uninstall(useSystemPath, dryRun bool) error {
	path := BinaryInstallPath(useSystemPath)

	if dryRun {
		return nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Already uninstalled
	}

	return os.Remove(path)
}

// GetInstalledVersion returns the version of the installed binary.
func GetInstalledVersion(useSystemPath bool) (string, error) {
	path := BinaryInstallPath(useSystemPath)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("binary not installed at %s", path)
	}

	// Try to get version by running the binary
	// For now, just return "installed" since we don't have a real binary to query
	return "installed", nil
}

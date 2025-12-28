package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ApplyOptions contains options for applying an update.
type ApplyOptions struct {
	// CurrentVersion is the current version.
	CurrentVersion string

	// Yes skips confirmation prompts.
	Yes bool

	// Insecure allows update without checksum verification.
	Insecure bool

	// DryRun shows what would be done without making changes.
	DryRun bool

	// OnProgress is called with progress updates.
	OnProgress func(step, message string)
}

// ApplyResult contains the result of applying an update.
type ApplyResult struct {
	Success        bool   `json:"success"`
	PreviousPath   string `json:"previous_path,omitempty"`
	BackupPath     string `json:"backup_path,omitempty"`
	NewPath        string `json:"new_path,omitempty"`
	OldVersion     string `json:"old_version"`
	NewVersion     string `json:"new_version"`
	ChecksumVerify bool   `json:"checksum_verified"`
	Error          string `json:"error,omitempty"`
	NeedsSudo      bool   `json:"needs_sudo,omitempty"`
	SudoCommand    string `json:"sudo_command,omitempty"`
	Steps          []ApplyStep
}

// ApplyStep represents a step in the update process.
type ApplyStep struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pending", "success", "failed", "skipped"
	Message string `json:"message,omitempty"`
}

// Apply downloads and applies an update.
func (c *Client) Apply(opts ApplyOptions) (*ApplyResult, error) {
	result := &ApplyResult{
		OldVersion: opts.CurrentVersion,
		Steps:      make([]ApplyStep, 0),
	}

	progress := opts.OnProgress
	if progress == nil {
		progress = func(step, message string) {}
	}

	// Step 1: Check for updates
	progress("check", "Checking for updates...")
	result.Steps = append(result.Steps, ApplyStep{Name: "Check for updates", Status: "pending"})

	checkResult, err := c.Check(opts.CurrentVersion)
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err.Error()
		return result, nil
	}

	if !checkResult.UpdateAvailable {
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = "Already up to date"
		result.Success = true
		result.NewVersion = checkResult.LatestVersion
		return result, nil
	}

	result.NewVersion = checkResult.LatestVersion
	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Update available: %s → %s", opts.CurrentVersion, checkResult.LatestVersion)

	// Check for matching asset
	if checkResult.DownloadURL == "" {
		result.Steps = append(result.Steps, ApplyStep{
			Name:    "Find matching asset",
			Status:  "failed",
			Message: fmt.Sprintf("No matching asset for %s/%s", runtime.GOOS, runtime.GOARCH),
		})
		result.Error = fmt.Sprintf("no matching release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
		return result, nil
	}

	// Step 2: Get current executable path
	progress("path", "Determining executable path...")
	result.Steps = append(result.Steps, ApplyStep{Name: "Get executable path", Status: "pending"})

	execPath, err := GetExecutablePath()
	if err != nil {
		result.Steps[len(result.Steps)-1].Status = "failed"
		result.Steps[len(result.Steps)-1].Message = err.Error()
		result.Error = err.Error()
		return result, nil
	}

	result.PreviousPath = execPath
	result.Steps[len(result.Steps)-1].Status = "success"
	result.Steps[len(result.Steps)-1].Message = execPath

	// Check if we can write to the directory
	if !opts.DryRun && !IsWritable(filepath.Dir(execPath)) {
		result.NeedsSudo = true
		result.SudoCommand = fmt.Sprintf("sudo mv <new-binary> %s", execPath)
		result.Steps = append(result.Steps, ApplyStep{
			Name:    "Check permissions",
			Status:  "failed",
			Message: "Cannot write to " + filepath.Dir(execPath),
		})
		result.Error = fmt.Sprintf("cannot write to %s - run with sudo or move the binary manually", filepath.Dir(execPath))
		return result, nil
	}

	// Step 3: Download new binary
	progress("download", "Downloading update...")
	result.Steps = append(result.Steps, ApplyStep{Name: "Download update", Status: "pending"})

	if opts.DryRun {
		result.Steps[len(result.Steps)-1].Status = "skipped"
		result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Would download from: %s", checkResult.DownloadURL)
	} else {
		tmpFile, err := downloadToTempFile(c.HTTPClient, checkResult.DownloadURL)
		if err != nil {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = err.Error()
			result.Error = err.Error()
			return result, nil
		}
		defer func() {
			if result.Error != "" {
				os.Remove(tmpFile)
			}
		}()

		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = "Downloaded to temp file"

		// Step 4: Verify checksum
		progress("verify", "Verifying checksum...")
		result.Steps = append(result.Steps, ApplyStep{Name: "Verify checksum", Status: "pending"})

		if checkResult.ChecksumURL != "" {
			checksumData, err := downloadData(c.HTTPClient, checkResult.ChecksumURL)
			if err != nil {
				result.Steps[len(result.Steps)-1].Status = "failed"
				result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Failed to download checksums: %v", err)
				if !opts.Insecure {
					result.Error = "checksum verification failed; use --insecure to skip"
					os.Remove(tmpFile)
					return result, nil
				}
			} else {
				entries, err := ParseChecksums(checksumData)
				if err != nil {
					result.Steps[len(result.Steps)-1].Status = "failed"
					result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Failed to parse checksums: %v", err)
					if !opts.Insecure {
						result.Error = "checksum verification failed; use --insecure to skip"
						os.Remove(tmpFile)
						return result, nil
					}
				} else {
					expectedHash, found := FindChecksum(entries, checkResult.AssetName)
					if !found {
						result.Steps[len(result.Steps)-1].Status = "failed"
						result.Steps[len(result.Steps)-1].Message = "Asset not found in checksums file"
						if !opts.Insecure {
							result.Error = "checksum verification failed; use --insecure to skip"
							os.Remove(tmpFile)
							return result, nil
						}
					} else {
						if err := VerifyChecksum(tmpFile, expectedHash); err != nil {
							result.Steps[len(result.Steps)-1].Status = "failed"
							result.Steps[len(result.Steps)-1].Message = err.Error()
							result.Error = err.Error()
							os.Remove(tmpFile)
							return result, nil
						}
						result.ChecksumVerify = true
						result.Steps[len(result.Steps)-1].Status = "success"
						result.Steps[len(result.Steps)-1].Message = "Checksum verified"
					}
				}
			}
		} else {
			if opts.Insecure {
				result.Steps[len(result.Steps)-1].Status = "skipped"
				result.Steps[len(result.Steps)-1].Message = "No checksums available (--insecure mode)"
			} else {
				result.Steps[len(result.Steps)-1].Status = "failed"
				result.Steps[len(result.Steps)-1].Message = "No checksums file in release"
				result.Error = "no checksums available; use --insecure to skip verification"
				os.Remove(tmpFile)
				return result, nil
			}
		}

		// Step 5: Safe swap
		progress("install", "Installing update...")
		result.Steps = append(result.Steps, ApplyStep{Name: "Install binary", Status: "pending"})

		backupPath, newPath, err := SafeSwap(tmpFile, execPath)
		if err != nil {
			result.Steps[len(result.Steps)-1].Status = "failed"
			result.Steps[len(result.Steps)-1].Message = err.Error()
			result.Error = err.Error()
			os.Remove(tmpFile)
			return result, nil
		}

		result.BackupPath = backupPath
		result.NewPath = newPath
		result.Steps[len(result.Steps)-1].Status = "success"
		result.Steps[len(result.Steps)-1].Message = fmt.Sprintf("Backup stored at: %s", backupPath)
	}

	if opts.DryRun {
		result.Steps = append(result.Steps, ApplyStep{
			Name:    "Install binary",
			Status:  "skipped",
			Message: "Would install to: " + execPath,
		})
	}

	result.Success = true
	return result, nil
}

// GetExecutablePath returns the path of the currently running executable.
func GetExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	return exe, nil
}

// IsWritable checks if a path is writable by the current user.
func IsWritable(path string) bool {
	// Try to create a temp file in the directory
	tmpPath := filepath.Join(path, ".mono-update-test")
	f, err := os.Create(tmpPath)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(tmpPath)
	return true
}

// SafeSwap performs a safe binary swap:
// 1. Write new binary to <path>.new
// 2. Make it executable
// 3. Move current to <path>.bak
// 4. Rename <path>.new to <path>
func SafeSwap(newBinaryPath, targetPath string) (backupPath, finalPath string, err error) {
	dir := filepath.Dir(targetPath)
	base := filepath.Base(targetPath)

	newPath := filepath.Join(dir, base+".new")
	backupPath = filepath.Join(dir, base+".bak")
	finalPath = targetPath

	// Step 1: Copy new binary to .new
	if err := copyFile(newBinaryPath, newPath); err != nil {
		return "", "", fmt.Errorf("failed to write new binary: %w", err)
	}

	// Step 2: Make executable
	if err := os.Chmod(newPath, 0755); err != nil {
		os.Remove(newPath)
		return "", "", fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Step 3: Move current to .bak (overwrite existing backup)
	if _, err := os.Stat(targetPath); err == nil {
		os.Remove(backupPath) // Remove old backup if exists
		if err := os.Rename(targetPath, backupPath); err != nil {
			os.Remove(newPath)
			return "", "", fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Step 4: Atomic rename .new to target
	if err := os.Rename(newPath, targetPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, targetPath)
		return "", "", fmt.Errorf("failed to install new binary: %w", err)
	}

	return backupPath, finalPath, nil
}

// downloadToTempFile downloads a URL to a temporary file.
func downloadToTempFile(client *http.Client, url string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "mono-commander")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "monoctl-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	return tmpFile.Name(), nil
}

// downloadData downloads a URL and returns the contents.
func downloadData(client *http.Client, url string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "mono-commander")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
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

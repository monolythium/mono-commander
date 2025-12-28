package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeSwap(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a "current" binary
	currentPath := filepath.Join(tmpDir, "monoctl")
	currentContent := []byte("current version")
	if err := os.WriteFile(currentPath, currentContent, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a "new" binary in temp
	newPath := filepath.Join(tmpDir, "monoctl.download")
	newContent := []byte("new version")
	if err := os.WriteFile(newPath, newContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Perform swap
	backupPath, finalPath, err := SafeSwap(newPath, currentPath)
	if err != nil {
		t.Fatalf("SafeSwap() error = %v", err)
	}

	// Verify backup was created
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("SafeSwap() should create backup file")
	}

	// Verify backup contains old content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}
	if string(backupContent) != string(currentContent) {
		t.Errorf("Backup content = %q, want %q", backupContent, currentContent)
	}

	// Verify new binary is in place
	newBinaryContent, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("Failed to read new binary: %v", err)
	}
	if string(newBinaryContent) != string(newContent) {
		t.Errorf("New binary content = %q, want %q", newBinaryContent, newContent)
	}

	// Verify new binary is executable
	info, err := os.Stat(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0100 == 0 {
		t.Error("New binary should be executable")
	}
}

func TestSafeSwapNoExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only a "new" binary, no existing binary
	targetPath := filepath.Join(tmpDir, "monoctl")
	newPath := filepath.Join(tmpDir, "monoctl.download")
	newContent := []byte("new version")
	if err := os.WriteFile(newPath, newContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Perform swap (should work even without existing binary)
	backupPath, finalPath, err := SafeSwap(newPath, targetPath)
	if err != nil {
		t.Fatalf("SafeSwap() error = %v", err)
	}

	// Backup path should be set but file shouldn't exist (no original to backup)
	_ = backupPath

	// Verify new binary is in place
	content, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("Failed to read new binary: %v", err)
	}
	if string(content) != string(newContent) {
		t.Errorf("Binary content = %q, want %q", content, newContent)
	}
}

func TestIsWritable(t *testing.T) {
	tmpDir := t.TempDir()

	// Temp dir should be writable
	if !IsWritable(tmpDir) {
		t.Error("Temp directory should be writable")
	}

	// Non-existent directory should not be writable
	if IsWritable("/nonexistent/directory/path") {
		t.Error("Non-existent directory should not be writable")
	}
}

func TestGetExecutablePath(t *testing.T) {
	path, err := GetExecutablePath()
	if err != nil {
		t.Fatalf("GetExecutablePath() error = %v", err)
	}

	// Should return a non-empty path
	if path == "" {
		t.Error("GetExecutablePath() returned empty path")
	}

	// Path should be absolute
	if !filepath.IsAbs(path) {
		t.Errorf("GetExecutablePath() returned non-absolute path: %s", path)
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "source")
	dst := filepath.Join(tmpDir, "dest")
	content := []byte("test content")

	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("Failed to read dest: %v", err)
	}

	if string(dstContent) != string(content) {
		t.Errorf("Copied content = %q, want %q", dstContent, content)
	}
}

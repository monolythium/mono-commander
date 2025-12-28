package logs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileSource_ReadLines(t *testing.T) {
	// Create temp file with test content
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := NewFileSource(logFile, false, 3)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := source.Lines(ctx)
	if err != nil {
		t.Fatalf("Lines() error = %v", err)
	}

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}

	// Should get last 3 lines
	if len(lines) != 3 {
		t.Errorf("Got %d lines, want 3", len(lines))
	}

	expected := []string{"line3", "line4", "line5"}
	for i, want := range expected {
		if i >= len(lines) {
			t.Errorf("Missing line %d", i)
			continue
		}
		if lines[i] != want {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], want)
		}
	}
}

func TestFileSource_ReadAllLines(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Request more lines than exist
	source := NewFileSource(logFile, false, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := source.Lines(ctx)
	if err != nil {
		t.Fatalf("Lines() error = %v", err)
	}

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}

	if len(lines) != 3 {
		t.Errorf("Got %d lines, want 3", len(lines))
	}
}

func TestFileSource_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "empty.log")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := NewFileSource(logFile, false, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := source.Lines(ctx)
	if err != nil {
		t.Fatalf("Lines() error = %v", err)
	}

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}

	if len(lines) != 0 {
		t.Errorf("Got %d lines for empty file, want 0", len(lines))
	}
}

func TestFileSource_FileNotFound(t *testing.T) {
	source := NewFileSource("/nonexistent/file.log", false, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := source.Lines(ctx)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestFileSource_Close(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(logFile, []byte("test\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	source := NewFileSource(logFile, false, 10)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := source.Lines(ctx)
	if err != nil {
		t.Fatalf("Lines() error = %v", err)
	}

	// Close should not error
	if err := source.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestNewJournalctlSource(t *testing.T) {
	source := NewJournalctlSource("test-unit", true, 50)

	if source.UnitName != "test-unit" {
		t.Errorf("UnitName = %q, want %q", source.UnitName, "test-unit")
	}
	if source.Follow != true {
		t.Errorf("Follow = %v, want %v", source.Follow, true)
	}
	if source.LineCount != 50 {
		t.Errorf("LineCount = %d, want %d", source.LineCount, 50)
	}
}

func TestNewFileSource(t *testing.T) {
	source := NewFileSource("/path/to/log", true, 100)

	if source.FilePath != "/path/to/log" {
		t.Errorf("FilePath = %q, want %q", source.FilePath, "/path/to/log")
	}
	if source.Follow != true {
		t.Errorf("Follow = %v, want %v", source.Follow, true)
	}
	if source.LineCount != 100 {
		t.Errorf("LineCount = %d, want %d", source.LineCount, 100)
	}
}

func TestGetLogSource_FileFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create logs subdirectory
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.Mkdir(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create log file
	logFile := filepath.Join(logsDir, "monod.log")
	if err := os.WriteFile(logFile, []byte("test log line\n"), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	source, err := GetLogSource("Localnet", tmpDir, false, 50)
	if err != nil {
		t.Fatalf("GetLogSource() error = %v", err)
	}
	defer source.Close()

	// Verify it's a FileSource
	fileSource, ok := source.(*FileSource)
	if !ok {
		t.Errorf("Expected FileSource, got %T", source)
	}
	if fileSource != nil && fileSource.FilePath != logFile {
		t.Errorf("FilePath = %q, want %q", fileSource.FilePath, logFile)
	}
}

func TestGetLogSource_AlternativeLocation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create log file in alternative location (no logs subdirectory)
	logFile := filepath.Join(tmpDir, "monod.log")
	if err := os.WriteFile(logFile, []byte("test log line\n"), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	source, err := GetLogSource("Localnet", tmpDir, false, 50)
	if err != nil {
		t.Fatalf("GetLogSource() error = %v", err)
	}
	defer source.Close()

	// Verify it's a FileSource
	fileSource, ok := source.(*FileSource)
	if !ok {
		t.Errorf("Expected FileSource, got %T", source)
	}
	if fileSource != nil && fileSource.FilePath != logFile {
		t.Errorf("FilePath = %q, want %q", fileSource.FilePath, logFile)
	}
}

func TestGetLogSource_NoLogFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Don't create any log files
	_, err := GetLogSource("Localnet", tmpDir, false, 50)
	if err == nil {
		t.Error("Expected error when no log file exists")
	}
	if !strings.Contains(err.Error(), "no log source available") {
		t.Errorf("Error message = %q, want to contain 'no log source available'", err.Error())
	}
}

func TestReadLastNLines(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Create file with many lines
	var content strings.Builder
	for i := 1; i <= 100; i++ {
		content.WriteString("line")
		content.WriteString(string(rune('0' + (i / 100))))
		content.WriteString(string(rune('0' + (i / 10 % 10))))
		content.WriteString(string(rune('0' + (i % 10))))
		content.WriteString("\n")
	}

	if err := os.WriteFile(logFile, []byte(content.String()), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	file, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	lines := readLastNLines(file, 5)

	if len(lines) != 5 {
		t.Errorf("Got %d lines, want 5", len(lines))
	}

	// Should be lines 96-100
	expected := []string{"line096", "line097", "line098", "line099", "line100"}
	for i, want := range expected {
		if i >= len(lines) {
			t.Errorf("Missing line %d", i)
			continue
		}
		if lines[i] != want {
			t.Errorf("lines[%d] = %q, want %q", i, lines[i], want)
		}
	}
}

func TestGetSystemdServiceStatus_NotLinux(t *testing.T) {
	// On non-Linux, this should return "N/A (not Linux)"
	// On Linux, it will try to check actual systemd status
	status := GetSystemdServiceStatus("Localnet")

	// Status should not be empty
	if status == "" {
		t.Error("GetSystemdServiceStatus() returned empty string")
	}
}

func TestJournalctlSource_Close(t *testing.T) {
	source := NewJournalctlSource("test-unit", false, 10)

	// Close on unstarted source should not error
	if err := source.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

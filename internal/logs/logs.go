// Package logs provides log tailing and journalctl helpers.
package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// LogSource represents a source of log lines.
type LogSource interface {
	// Lines returns a channel of log lines.
	Lines(ctx context.Context) (<-chan string, error)
	// Close closes the log source.
	Close() error
}

// JournalctlSource reads logs from journalctl.
type JournalctlSource struct {
	UnitName  string
	Follow    bool
	LineCount int
	cmd       *exec.Cmd
}

// NewJournalctlSource creates a new journalctl log source.
func NewJournalctlSource(unitName string, follow bool, lineCount int) *JournalctlSource {
	return &JournalctlSource{
		UnitName:  unitName,
		Follow:    follow,
		LineCount: lineCount,
	}
}

// Lines returns a channel of log lines from journalctl.
func (j *JournalctlSource) Lines(ctx context.Context) (<-chan string, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("journalctl not available on %s", runtime.GOOS)
	}

	args := []string{"-u", j.UnitName, "--no-pager"}
	if j.LineCount > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", j.LineCount))
	}
	if j.Follow {
		args = append(args, "-f")
	}

	j.cmd = exec.CommandContext(ctx, "journalctl", args...)
	stdout, err := j.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := j.cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start journalctl: %w", err)
	}

	ch := make(chan string, 100)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case ch <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// Close stops the journalctl process.
func (j *JournalctlSource) Close() error {
	if j.cmd != nil && j.cmd.Process != nil {
		return j.cmd.Process.Kill()
	}
	return nil
}

// FileSource reads logs from a file.
type FileSource struct {
	FilePath  string
	Follow    bool
	LineCount int
	file      *os.File
}

// NewFileSource creates a new file log source.
func NewFileSource(filePath string, follow bool, lineCount int) *FileSource {
	return &FileSource{
		FilePath:  filePath,
		Follow:    follow,
		LineCount: lineCount,
	}
}

// Lines returns a channel of log lines from a file.
func (f *FileSource) Lines(ctx context.Context) (<-chan string, error) {
	file, err := os.Open(f.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	f.file = file

	ch := make(chan string, 100)

	go func() {
		defer close(ch)
		defer file.Close()

		// If lineCount > 0, seek to show last N lines
		if f.LineCount > 0 {
			lastLines := readLastNLines(file, f.LineCount)
			for _, line := range lastLines {
				select {
				case ch <- line:
				case <-ctx.Done():
					return
				}
			}
			// Reset to end for following
			file.Seek(0, io.SeekEnd)
		}

		if !f.Follow {
			return
		}

		// Follow mode: tail the file
		reader := bufio.NewReader(file)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						time.Sleep(100 * time.Millisecond)
						continue
					}
					return
				}
				ch <- strings.TrimSuffix(line, "\n")
			}
		}
	}()

	return ch, nil
}

// Close closes the file.
func (f *FileSource) Close() error {
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

// readLastNLines reads the last N lines from a file.
func readLastNLines(file *os.File, n int) []string {
	const bufSize = 1024
	stat, err := file.Stat()
	if err != nil {
		return nil
	}

	size := stat.Size()
	var lines []string
	var leftover string

	for offset := int64(bufSize); offset <= size+bufSize; offset += bufSize {
		readStart := size - offset
		if readStart < 0 {
			readStart = 0
		}

		readSize := offset
		if offset > size {
			readSize = size
		}
		if readStart == 0 {
			readSize = size - (offset - bufSize)
		}

		buf := make([]byte, readSize)
		file.Seek(readStart, io.SeekStart)
		file.Read(buf)

		chunk := string(buf) + leftover
		parts := strings.Split(chunk, "\n")

		if readStart > 0 {
			leftover = parts[0]
			parts = parts[1:]
		} else {
			leftover = ""
		}

		// Prepend lines
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" {
				lines = append([]string{parts[i]}, lines...)
			}
		}

		if len(lines) >= n {
			break
		}
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	// Reset file position
	file.Seek(0, io.SeekStart)
	return lines
}

// GetLogSource returns the appropriate log source for the current platform.
func GetLogSource(network string, home string, follow bool, lines int) (LogSource, error) {
	// Try journalctl first on Linux
	if runtime.GOOS == "linux" {
		// Try different unit name patterns
		unitNames := []string{
			fmt.Sprintf("monod-%s", network),
			fmt.Sprintf("monod@%s", network),
			"monod",
		}

		for _, unit := range unitNames {
			if checkJournalctlUnit(unit) {
				return NewJournalctlSource(unit, follow, lines), nil
			}
		}
	}

	// Fall back to file
	logFile := filepath.Join(home, "logs", "monod.log")
	if _, err := os.Stat(logFile); err == nil {
		return NewFileSource(logFile, follow, lines), nil
	}

	// Try alternative log location
	logFile = filepath.Join(home, "monod.log")
	if _, err := os.Stat(logFile); err == nil {
		return NewFileSource(logFile, follow, lines), nil
	}

	return nil, fmt.Errorf("no log source available (tried journalctl and file)")
}

// checkJournalctlUnit checks if a journalctl unit exists.
func checkJournalctlUnit(unit string) bool {
	cmd := exec.Command("systemctl", "is-active", unit)
	err := cmd.Run()
	return err == nil
}

// GetSystemdServiceStatus returns the status of a systemd service.
func GetSystemdServiceStatus(network string) string {
	if runtime.GOOS != "linux" {
		return "N/A (not Linux)"
	}

	unitNames := []string{
		fmt.Sprintf("monod-%s", network),
		fmt.Sprintf("monod@%s", network),
		"monod",
	}

	for _, unit := range unitNames {
		cmd := exec.Command("systemctl", "is-active", unit)
		output, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output))
		}
	}

	return "not found"
}

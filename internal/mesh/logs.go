// Package mesh provides Mesh/Rosetta API sidecar management for mono-commander.
package mesh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
)

// LogsOptions contains options for tailing logs.
type LogsOptions struct {
	// Network is the network name.
	Network string

	// Follow enables continuous log tailing.
	Follow bool

	// Lines is the number of lines to show.
	Lines int
}

// LogsSource provides log lines from the mesh service.
type LogsSource struct {
	cmd    *exec.Cmd
	reader io.ReadCloser
}

// GetLogSource returns a log source for the mesh service.
func GetLogSource(opts LogsOptions) (*LogsSource, error) {
	unitName := UnitName(opts.Network)

	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("log tailing is only supported on Linux with systemd")
	}

	// Check if journalctl is available
	if _, err := exec.LookPath("journalctl"); err != nil {
		return nil, fmt.Errorf("journalctl not available: %w", err)
	}

	// Build journalctl command
	args := []string{"-u", unitName}

	if opts.Lines > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", opts.Lines))
	} else {
		args = append(args, "-n", "50") // Default to 50 lines
	}

	if opts.Follow {
		args = append(args, "-f")
	}

	args = append(args, "--no-pager")

	cmd := exec.Command("journalctl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start journalctl: %w", err)
	}

	return &LogsSource{
		cmd:    cmd,
		reader: stdout,
	}, nil
}

// Lines returns a channel that yields log lines.
func (ls *LogsSource) Lines(ctx context.Context) (<-chan string, error) {
	lines := make(chan string, 100)

	go func() {
		defer close(lines)
		defer ls.Close()

		scanner := bufio.NewScanner(ls.reader)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case lines <- scanner.Text():
			}
		}
	}()

	return lines, nil
}

// Close closes the log source.
func (ls *LogsSource) Close() error {
	if ls.cmd != nil && ls.cmd.Process != nil {
		ls.cmd.Process.Kill()
		ls.cmd.Wait()
	}
	return nil
}

// GetRecentLogs returns recent log lines as a slice.
func GetRecentLogs(network string, lines int) ([]string, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("log retrieval is only supported on Linux with systemd")
	}

	unitName := UnitName(network)
	args := []string{"-u", unitName, "-n", fmt.Sprintf("%d", lines), "--no-pager"}

	cmd := exec.Command("journalctl", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	var result []string
	reader := &stringReader{data: string(out)}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}

	return result, nil
}

// stringScanner wraps a string as a scanner
type stringReader struct {
	data string
	pos  int
}

func (s *stringReader) Read(p []byte) (n int, err error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	n = copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}

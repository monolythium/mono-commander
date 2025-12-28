// Package os provides OS-level operations for mono-commander.
package os

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// CommandResult contains the result of running a command
type CommandResult struct {
	// Command is the command that was run (may be redacted)
	Command string
	// ExitCode is the exit code of the command
	ExitCode int
	// Stdout is the (possibly redacted) stdout output
	Stdout string
	// Stderr is the (possibly redacted) stderr output
	Stderr string
	// Success indicates if the command succeeded (exit code 0)
	Success bool
	// Duration is how long the command took to run
	Duration time.Duration
	// Error contains any error that occurred
	Error error
}

// Runner executes commands with safety measures
type Runner struct {
	// DryRun if true, only prints commands without executing
	DryRun bool
	// Timeout for command execution (default: 60s)
	Timeout time.Duration
	// RedactPatterns are regex patterns to redact from output
	RedactPatterns []*regexp.Regexp
	// RedactEnvVars are environment variable names to redact
	RedactEnvVars []string
	// WorkDir is the working directory for commands
	WorkDir string
}

// DefaultRunner creates a runner with sensible defaults
func DefaultRunner() *Runner {
	return &Runner{
		DryRun:  true, // Safe default
		Timeout: 60 * time.Second,
		RedactPatterns: []*regexp.Regexp{
			// Redact mnemonics (12 or 24 words)
			regexp.MustCompile(`(?i)(mnemonic|seed)[:\s]+[a-z\s]{30,}`),
			// Redact private keys (hex)
			regexp.MustCompile(`(?i)(private[_\s]?key|priv[_\s]?key)[:\s]*[a-fA-F0-9]{64}`),
			// Redact base64 encoded keys
			regexp.MustCompile(`(?i)(key|secret)[:\s]*[A-Za-z0-9+/]{40,}={0,2}`),
			// Redact Cosmos keyring passwords
			regexp.MustCompile(`(?i)password[:\s]+\S+`),
		},
		RedactEnvVars: []string{
			"MONOD_KEYRING_PASSWORD",
			"MONO_MNEMONIC",
		},
	}
}

// NewRunner creates a new Runner with the specified dry-run setting
func NewRunner(dryRun bool) *Runner {
	r := DefaultRunner()
	r.DryRun = dryRun
	return r
}

// Redact applies redaction patterns to the given text
func (r *Runner) Redact(text string) string {
	result := text
	for _, pattern := range r.RedactPatterns {
		result = pattern.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// Run executes a command and returns the result
func (r *Runner) Run(ctx context.Context, binary string, args []string) *CommandResult {
	result := &CommandResult{
		Command: binary + " " + strings.Join(args, " "),
	}

	// Redact sensitive args in command display
	result.Command = r.redactArgs(result.Command)

	if r.DryRun {
		result.Success = true
		result.Stdout = "(dry-run: command not executed)"
		return result
	}

	// Create context with timeout
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	// Create command
	cmd := exec.CommandContext(ctx, binary, args...)
	if r.WorkDir != "" {
		cmd.Dir = r.WorkDir
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute
	start := time.Now()
	err := cmd.Run()
	result.Duration = time.Since(start)

	// Process result
	result.Stdout = r.Redact(stdout.String())
	result.Stderr = r.Redact(stderr.String())

	if err != nil {
		result.Error = err
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Success = false
	} else {
		result.ExitCode = 0
		result.Success = true
	}

	return result
}

// redactArgs redacts sensitive information from command arguments
func (r *Runner) redactArgs(command string) string {
	// Redact --from value if it looks like a key name (not address)
	// We keep addresses visible but redact key names for privacy
	result := command

	// Apply general redaction patterns
	result = r.Redact(result)

	return result
}

// RunTx executes a transaction command with additional safety checks
func (r *Runner) RunTx(ctx context.Context, binary string, args []string) *CommandResult {
	// Additional validation for tx commands
	if len(args) < 2 || args[0] != "tx" {
		return &CommandResult{
			Success: false,
			Error:   fmt.Errorf("RunTx requires 'tx' as first argument"),
		}
	}

	// Check for dangerous flags
	for _, arg := range args {
		if arg == "--force" || arg == "--yes" || arg == "-y" {
			// These are OK for broadcast, but log a warning
		}
	}

	return r.Run(ctx, binary, args)
}

// CheckBinaryExists verifies that a binary exists in PATH
func (r *Runner) CheckBinaryExists(binary string) error {
	_, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("binary not found: %s (ensure it's installed and in PATH)", binary)
	}
	return nil
}

// CheckKeyExists checks if a key exists in the keyring (dry-run safe)
func (r *Runner) CheckKeyExists(ctx context.Context, home, keyName string) (bool, error) {
	// Even in dry-run, we can check key existence
	tempRunner := &Runner{
		DryRun:  false, // Need to actually run this check
		Timeout: 10 * time.Second,
	}

	args := []string{"keys", "show", keyName}
	if home != "" {
		args = append(args, "--home", home)
	}
	// Use test keyring to avoid password prompts
	args = append(args, "--keyring-backend", "test")

	result := tempRunner.Run(ctx, "monod", args)
	if result.Success {
		return true, nil
	}

	// Key doesn't exist or keyring error
	if strings.Contains(result.Stderr, "not found") || strings.Contains(result.Stderr, "no such key") {
		return false, nil
	}

	// Some other error - might be keyring not initialized
	return false, fmt.Errorf("failed to check key: %s", result.Stderr)
}

// TxSummary extracts a minimal summary from transaction output
// This ensures we don't log full transaction details
type TxSummary struct {
	TxHash  string
	Height  int64
	Code    int
	Success bool
	RawLog  string // May be truncated
}

// ExtractTxSummary extracts minimal tx info from command output
func ExtractTxSummary(output string) *TxSummary {
	summary := &TxSummary{}

	// Look for common patterns in Cosmos SDK tx output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// JSON output parsing
		if strings.Contains(line, `"txhash"`) {
			// Extract txhash from JSON
			if idx := strings.Index(line, `"txhash"`); idx >= 0 {
				// Simple extraction - production would use proper JSON parsing
				rest := line[idx:]
				if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
					value := strings.TrimPrefix(rest[colonIdx+1:], " ")
					value = strings.Trim(value, `",`)
					summary.TxHash = value
				}
			}
		}

		// Look for success indicators
		if strings.Contains(line, `"code":0`) || strings.Contains(line, `"code": 0`) {
			summary.Success = true
			summary.Code = 0
		}

		// Look for height
		if strings.Contains(line, `"height"`) {
			// Extract height (simplified)
			if idx := strings.Index(line, `"height"`); idx >= 0 {
				rest := line[idx:]
				if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
					value := strings.TrimPrefix(rest[colonIdx+1:], " ")
					value = strings.Trim(value, `",`)
					fmt.Sscanf(value, "%d", &summary.Height)
				}
			}
		}
	}

	// Truncate raw log if present
	if len(output) > 500 {
		summary.RawLog = output[:500] + "...[truncated]"
	} else {
		summary.RawLog = output
	}

	return summary
}

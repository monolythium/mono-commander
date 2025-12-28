package os

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDefaultRunner(t *testing.T) {
	r := DefaultRunner()

	if !r.DryRun {
		t.Error("DefaultRunner() should have DryRun=true for safety")
	}

	if r.Timeout != 60*time.Second {
		t.Errorf("DefaultRunner() Timeout = %v, want 60s", r.Timeout)
	}

	if len(r.RedactPatterns) == 0 {
		t.Error("DefaultRunner() should have redaction patterns")
	}
}

func TestNewRunner(t *testing.T) {
	r := NewRunner(false)

	if r.DryRun {
		t.Error("NewRunner(false) should have DryRun=false")
	}

	r2 := NewRunner(true)
	if !r2.DryRun {
		t.Error("NewRunner(true) should have DryRun=true")
	}
}

func TestRunner_Redact(t *testing.T) {
	r := DefaultRunner()

	tests := []struct {
		name  string
		input string
		want  string // Should not contain these
	}{
		{
			name:  "mnemonic",
			input: "mnemonic: abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			want:  "mnemonic",
		},
		{
			name:  "private key",
			input: "private_key: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:  "[REDACTED]",
		},
		{
			name:  "password",
			input: "password: mysecretpassword123",
			want:  "[REDACTED]",
		},
		{
			name:  "normal text",
			input: "This is normal log output with height=12345",
			want:  "height=12345", // Should NOT be redacted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)

			// For normal text, it should be preserved
			if tt.name == "normal text" {
				if result != tt.input {
					t.Errorf("Redact() should not modify normal text: got %q", result)
				}
				return
			}

			// For sensitive data, it should be redacted
			if strings.Contains(result, "abandon") || strings.Contains(result, "mysecretpassword") {
				t.Errorf("Redact() failed to redact sensitive data: %q", result)
			}
		})
	}
}

func TestRunner_Run_DryRun(t *testing.T) {
	r := DefaultRunner()
	ctx := context.Background()

	result := r.Run(ctx, "echo", []string{"hello"})

	if !result.Success {
		t.Error("Run() dry-run should be successful")
	}

	if !strings.Contains(result.Stdout, "dry-run") {
		t.Errorf("Run() dry-run output should mention dry-run: %q", result.Stdout)
	}

	if result.ExitCode != 0 {
		t.Errorf("Run() dry-run ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestRunner_Run_ActualCommand(t *testing.T) {
	r := NewRunner(false) // Not dry-run
	r.Timeout = 5 * time.Second
	ctx := context.Background()

	result := r.Run(ctx, "echo", []string{"hello", "world"})

	if !result.Success {
		t.Errorf("Run() echo should succeed: %v", result.Error)
	}

	if !strings.Contains(result.Stdout, "hello world") {
		t.Errorf("Run() output = %q, want to contain 'hello world'", result.Stdout)
	}

	if result.Duration == 0 {
		t.Error("Run() should record duration")
	}
}

func TestRunner_Run_NonExistentCommand(t *testing.T) {
	r := NewRunner(false)
	r.Timeout = 2 * time.Second
	ctx := context.Background()

	result := r.Run(ctx, "nonexistent_command_12345", []string{})

	if result.Success {
		t.Error("Run() nonexistent command should fail")
	}

	if result.Error == nil {
		t.Error("Run() nonexistent command should have error")
	}
}

func TestRunner_RunTx_ValidatesArgs(t *testing.T) {
	r := NewRunner(true)
	ctx := context.Background()

	// Invalid: not starting with "tx"
	result := r.RunTx(ctx, "monod", []string{"status"})

	if result.Success {
		t.Error("RunTx() should fail for non-tx command")
	}

	if result.Error == nil {
		t.Error("RunTx() should have error for non-tx command")
	}
}

func TestRunner_RunTx_ValidTxCommand(t *testing.T) {
	r := NewRunner(true) // Dry-run
	ctx := context.Background()

	result := r.RunTx(ctx, "monod", []string{"tx", "staking", "delegate"})

	if !result.Success {
		t.Error("RunTx() dry-run should succeed")
	}
}

func TestRunner_CheckBinaryExists(t *testing.T) {
	r := DefaultRunner()

	// echo should exist on all systems
	err := r.CheckBinaryExists("echo")
	if err != nil {
		t.Errorf("CheckBinaryExists(echo) should succeed: %v", err)
	}

	// nonexistent should fail
	err = r.CheckBinaryExists("nonexistent_binary_12345")
	if err == nil {
		t.Error("CheckBinaryExists(nonexistent) should fail")
	}
}

func TestExtractTxSummary(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantTxHash string
		wantCode   int
	}{
		{
			name: "success with txhash",
			output: `{
				"txhash": "ABCD1234567890",
				"code": 0,
				"height": "12345"
			}`,
			wantTxHash: "ABCD1234567890",
			wantCode:   0,
		},
		{
			name:       "empty output",
			output:     "",
			wantTxHash: "",
			wantCode:   0,
		},
		{
			name:       "no json",
			output:     "Transaction submitted successfully",
			wantTxHash: "",
			wantCode:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := ExtractTxSummary(tt.output)

			if summary.TxHash != tt.wantTxHash {
				t.Errorf("ExtractTxSummary() TxHash = %q, want %q", summary.TxHash, tt.wantTxHash)
			}

			if summary.Code != tt.wantCode {
				t.Errorf("ExtractTxSummary() Code = %d, want %d", summary.Code, tt.wantCode)
			}
		})
	}
}

func TestExtractTxSummary_TruncatesLongOutput(t *testing.T) {
	// Create output longer than 500 chars
	longOutput := strings.Repeat("a", 600)

	summary := ExtractTxSummary(longOutput)

	if len(summary.RawLog) > 520 { // 500 + "[truncated]" message
		t.Errorf("ExtractTxSummary() should truncate long output, got len=%d", len(summary.RawLog))
	}

	if !strings.Contains(summary.RawLog, "[truncated]") {
		t.Error("ExtractTxSummary() should indicate truncation")
	}
}

func TestCommandResult_Fields(t *testing.T) {
	result := &CommandResult{
		Command:  "monod tx staking delegate",
		ExitCode: 0,
		Stdout:   "success",
		Stderr:   "",
		Success:  true,
		Duration: 100 * time.Millisecond,
	}

	if result.ExitCode != 0 {
		t.Errorf("CommandResult.ExitCode = %d, want 0", result.ExitCode)
	}

	if !result.Success {
		t.Error("CommandResult.Success should be true")
	}

	if result.Duration == 0 {
		t.Error("CommandResult.Duration should be set")
	}
}

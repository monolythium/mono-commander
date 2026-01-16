// Package core provides the core logic for mono-commander.
package core

import (
	"strings"
	"testing"
)

func TestBuildBankSendTx(t *testing.T) {
	tests := []struct {
		name    string
		opts    TxBuilderOptions
		params  BankSendParams
		wantErr bool
	}{
		{
			name: "valid bank send",
			opts: TxBuilderOptions{
				Network: NetworkSprintnet,
				From:    "operator4",
				Home:    "/home/claude/.monod",
			},
			params: BankSendParams{
				ToAddress: "mono1y84l2xpqj3w4e6fdkaatryslgnzc8034aq38vq",
				Amount:    "210000000000000000000000alyth",
			},
			wantErr: false,
		},
		{
			name: "invalid recipient address",
			opts: TxBuilderOptions{
				Network: NetworkSprintnet,
				From:    "operator4",
			},
			params: BankSendParams{
				ToAddress: "invalid_address",
				Amount:    "210000000000000000000000alyth",
			},
			wantErr: true,
		},
		{
			name: "invalid amount",
			opts: TxBuilderOptions{
				Network: NetworkSprintnet,
				From:    "operator4",
			},
			params: BankSendParams{
				ToAddress: "mono1y84l2xpqj3w4e6fdkaatryslgnzc8034aq38vq",
				Amount:    "invalid",
			},
			wantErr: true,
		},
		{
			name: "wrong denom (LYTH instead of alyth)",
			opts: TxBuilderOptions{
				Network: NetworkSprintnet,
				From:    "operator4",
			},
			params: BankSendParams{
				ToAddress: "mono1y84l2xpqj3w4e6fdkaatryslgnzc8034aq38vq",
				Amount:    "210000LYTH",
			},
			wantErr: true,
		},
		{
			name: "empty recipient",
			opts: TxBuilderOptions{
				Network: NetworkSprintnet,
				From:    "operator4",
			},
			params: BankSendParams{
				ToAddress: "",
				Amount:    "210000000000000000000000alyth",
			},
			wantErr: true,
		},
		{
			name: "zero amount",
			opts: TxBuilderOptions{
				Network: NetworkSprintnet,
				From:    "operator4",
			},
			params: BankSendParams{
				ToAddress: "mono1y84l2xpqj3w4e6fdkaatryslgnzc8034aq38vq",
				Amount:    "0alyth",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := BuildBankSendTx(tt.opts, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildBankSendTx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && cmd == nil {
				t.Error("BuildBankSendTx() returned nil command without error")
			}
			if !tt.wantErr && cmd != nil {
				// Verify command structure
				if cmd.Action != TxActionSend {
					t.Errorf("BuildBankSendTx() action = %v, want %v", cmd.Action, TxActionSend)
				}
				if cmd.Binary != "monod" {
					t.Errorf("BuildBankSendTx() binary = %v, want monod", cmd.Binary)
				}
				// Verify the command string contains expected parts
				cmdStr := cmd.String()
				if !strings.Contains(cmdStr, "tx bank send") {
					t.Errorf("BuildBankSendTx() command should contain 'tx bank send', got %s", cmdStr)
				}
				if !strings.Contains(cmdStr, tt.params.ToAddress) {
					t.Errorf("BuildBankSendTx() command should contain recipient %s, got %s", tt.params.ToAddress, cmdStr)
				}
				if !strings.Contains(cmdStr, tt.params.Amount) {
					t.Errorf("BuildBankSendTx() command should contain amount %s, got %s", tt.params.Amount, cmdStr)
				}
			}
		})
	}
}

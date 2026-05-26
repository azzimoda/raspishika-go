package model_test

import (
	"testing"
	"time"

	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/testutil"
)

func TestChat_GetState(t *testing.T) {
	testDir := t.TempDir()
	testutil.InitConfig(t, testDir)

	tests := []struct {
		name        string // description of this test case
		chat        model.Chat
		wantState   model.ChatState
		wantExpired bool
	}{
		{
			name: "chat with fresh default state",
			chat: model.Chat{
				State:     model.ChatStateDefault,
				UpdatedAt: time.Now(),
			},
			wantState:   model.ChatStateDefault,
			wantExpired: false,
		},
		{
			name: "chat with expired default state",
			chat: model.Chat{
				State:     model.ChatStateDefault,
				UpdatedAt: time.Now().Add(11 * time.Minute),
			},
			wantState:   model.ChatStateDefault,
			wantExpired: false,
		},
		{
			name: "chat with fresh selecting_group state",
			chat: model.Chat{
				State:     model.ChatStateSelectingGroup,
				UpdatedAt: time.Now(),
			},
			wantState:   model.ChatStateSelectingGroup,
			wantExpired: false,
		},
		{
			name: "chat with expired selecting_group state",
			chat: model.Chat{
				State:     model.ChatStateSelectingGroup,
				UpdatedAt: time.Now().Add(-11 * time.Minute),
			},
			wantState:   model.ChatStateDefault,
			wantExpired: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldState := tt.chat.State
			gotState, gotExpired := tt.chat.GetState()
			if gotState != tt.wantState || gotExpired != tt.wantExpired {
				t.Errorf("(%v).GetState() = (%v, %v), want (%v, %v)", tt.chat.WithState(oldState), gotState, gotExpired, tt.wantState, tt.wantExpired)
				/*
				 * chat_test.go:44:
				 * ({0 0 <nil> default <nil> <nil> <nil> false false 0 false 0001-01-01 00:00:00 +0000 UTC 2026-05-26 20:05:22.559641022 +0500 +05 m=+660.000812752})
				 * 	.GetState() = (default, false), want (default, true)
				 *
				 */
			}
		})
	}
}

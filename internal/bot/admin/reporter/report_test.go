package reporter_test

import(
	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	"github.com/go-telegram/bot"
	"testing"
)

func TestReportConfig_Msg(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		bot             *bot.Bot
		recipientChatID int64
		// Named input parameters for target function.
		text    string
		want    *reporter.Report
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := reporter.NewReportConfig(tt.bot, tt.recipientChatID)
			got, gotErr := rc.Msg(tt.text)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Msg() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Msg() succeeded unexpectedly")
			}
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("Msg() = %v, want %v", got, tt.want)
			}
		})
	}
}

package sendings

import (
	"testing"
	"time"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/services"
)

var debugConfigFile string
var templateFile string

func init() {
	debugConfigFile, templateFile = utils.InitPaths()
	println(debugConfigFile, templateFile)
}

func TestSendingManager_sendPairNotificationToGroup(t *testing.T) {
	var Times = []string{"7:45", "9:30", "11:15", "13:30", "15:15", "17:00", "18:45"}

	testDir := t.TempDir()
	cfg := utils.InitConfig(t, debugConfigFile, templateFile, testDir)

	srvs, err := services.NewServices(cfg)
	if err != nil {
		t.Fatal(err)
	}

	b, err := mainbot.New(cfg, nil, srvs)
	if err != nil {
		t.Fatal(err)
	}

	sm := NewSendingManager(cfg, b, srvs)

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		groupName     string
		chats         []*database.Chat
		wantErrs      bool
		wantFailedAll bool
	}{
		{"send pair sending to 1 group with 1 chat",
			"ИСПт-22-(9)-2",
			[]*database.Chat{{TgChatID: cfg.Telegram.AdminID, PairSending: true}},
			false,
			false,
		},
	}
	for _, tt := range tests {
		for _, timeStr := range Times {
			sendingTime, _ := time.Parse("15:04", timeStr)
			sendingTime = sendingTime.Add(time.Minute)

			t.Run(tt.name+" "+timeStr, func(t *testing.T) {
				gotErrs, failedAll := sm.sendPairNotificationToGroup(tt.groupName, sendingTime, tt.chats)
				if tt.wantErrs != (len(gotErrs) > 0) {
					t.Errorf("sendPairNotificationToGroup() = %v, want %v", gotErrs, tt.wantErrs)
				}
				if tt.wantFailedAll != failedAll {
					t.Errorf("sendPairNotificationToGroup() = %v, want %v", failedAll, tt.wantFailedAll)
				}
			})
		}
	}
}

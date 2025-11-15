package bot

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/logger"
)

var debugConfigFile string
var templateFile string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(filepath.Dir(filename), "..", "..", "..")

	debugConfigFile = filepath.Join(rootDir, "configs/.debug-config.yml")
	templateFile = filepath.Join(rootDir, "storage/schedule_template.html")
}

func TestBot_processDailySending(t *testing.T) {
	// Initialize services.
	testsDir, cfg, repo, browser, cache := utils.InitServices(t, debugConfigFile, templateFile)
	b, err := New(cfg, nil, repo, browser, cache)
	if err != nil {
		t.Fatalf("could not construct receiver type: %v", err)
	}

	// Prepare database.
	if _, err := scraper.FetchGroups(repo, browser, cache); err != nil {
		t.Fatalf("could not fetch groups: %v", err)
	}

	// Test
	groupNames := []string{
		"ИСПт-22-(9)-1",
		"ИСПт-22-(9)-2",
		"ИСПт-23-(9)-1",
		"ИСПт-24-(9)-1",
		"ИСПт-25-(9)-1",
	}
	now := time.Now()
	timeStr := now.Format("15:04")
	tests := []struct {
		name string // description of this test case

		chats []database.Chat
	}{
		{"send daily schedule", []database.Chat{
			{TgChatID: cfg.Telegram.AdminID, GroupName: &groupNames[1], DailySendingTime: &timeStr},
		}},
		{"send daily schedule to groups", []database.Chat{
			{TgChatID: cfg.Telegram.AdminID, GroupName: &groupNames[0], DailySendingTime: &timeStr},
			{TgChatID: 0, GroupName: &groupNames[1], DailySendingTime: &timeStr}, // Fake chat IDs just to test fetching.
			{TgChatID: 1, GroupName: &groupNames[2], DailySendingTime: &timeStr},
			{TgChatID: 2, GroupName: &groupNames[3], DailySendingTime: &timeStr},
			{TgChatID: 3, GroupName: &groupNames[4], DailySendingTime: &timeStr},
		}},
	}

	for _, tt := range tests {
		if err := repo.DeleteAllChats(); err != nil {
			t.Fatalf("failed to delete all chats: %v", err)
		}

		if err := repo.InsertChats(tt.chats); err != nil {
			t.Fatalf("failed to insert chats: %v", err)
		}

		t.Run(tt.name, func(t *testing.T) {
			b.processDailySending(now)
		})
	}

	utils.Cleanup(t, testsDir)
}

func TestBot_processPairSending(t *testing.T) {
	var Times = []string{"7:45", "9:30", "11:15", "13:30", "15:15", "17:00", "18:45"}

	// Initialize services.
	testsDir, cfg, repo, browser, cache := utils.InitServices(t, debugConfigFile, templateFile)
	logger.SetupLogger(config.LoggerConfig{Level: "trace", Dir: ""})

	b, err := New(cfg, nil, repo, browser, cache)
	if err != nil {
		t.Fatalf("could not construct receiver type: %v", err)
	}

	// Prepare database.
	groups, err := scraper.FetchGroups(repo, browser, cache)
	if err != nil {
		t.Fatalf("could not fetch groups: %v", err)
	}

	groupName := groups[1].GroupName
	fakeGroupName := "КИПр-23-(9)-1"

	// Test.
	tests := []struct {
		name  string // description of this test case
		chats []database.Chat
	}{
		{"send pair schedule to exiting chat for exiting group", []database.Chat{
			{TgChatID: cfg.Telegram.AdminID, GroupName: &groupName, PairSending: true},
		}},
		{"send pair schedule to exiting chat for fake group", []database.Chat{
			{TgChatID: cfg.Telegram.AdminID, GroupName: &fakeGroupName, PairSending: true},
		}},
	}

	for _, tt := range tests {
		if err := repo.DeleteAllChats(); err != nil {
			t.Fatalf("failed to delete all chats: %v", err)
		}
		if err := repo.InsertChats(tt.chats); err != nil {
			t.Fatalf("failed to insert chats: %v", err)
		}

		for _, timeStr := range Times {
			startTime, err := time.Parse("15:04", timeStr)
			if err != nil {
				t.Fatalf("could not parse time: %v", err)
			}

			t.Run(tt.name+" for time "+timeStr, func(t *testing.T) {
				b.processPairSending(startTime)
			})
		}
	}

	// Wait for deleting the messages.
	time.Sleep(10 * time.Second)

	utils.Cleanup(t, testsDir)
}

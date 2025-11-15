package bot

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/logger"
)

var debugConfigFile string
var templateFile string
var testsDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(filepath.Dir(filename), "..", "..", "..")

	debugConfigFile = filepath.Join(rootDir, "configs/.debug-config.yml")
	templateFile = filepath.Join(rootDir, "storage/schedule_template.html")

	testsDir = filepath.Join(rootDir, ".tests")
	// Delete and recreate tests dir.
	if err := os.RemoveAll(testsDir); err != nil {
		log.Fatal().Err(err).Msg("could not delete tests dir")
	}
	if err := os.MkdirAll(testsDir, 0755); err != nil {
		log.Fatal().Err(err).Msg("could not create tests dir")
	}
}

func TestBot_processDailySending(t *testing.T) {
	// Initialize services.
	cfg, repo, browser, cache := utils.InitServices(t, debugConfigFile, templateFile, testsDir)
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
	cfg, repo, browser, cache := utils.InitServices(t, debugConfigFile, templateFile, testsDir)
	logger.SetupLogger(config.LoggerConfig{Level: "trace", Dir: ""})

	b, err := New(cfg, nil, repo, browser, cache)
	if err != nil {
		t.Fatalf("could not construct receiver type: %v", err)
	}

	// Prepare database.
	groupName := "ИСПт-22-(9)-2"
	chats := []database.Chat{{TgChatID: cfg.Telegram.AdminID, GroupName: &groupName, PairSending: true}}
	if err := repo.InsertChats(chats); err != nil {
		t.Fatalf("could not insert chats: %v", err)
	}

	if _, err := scraper.FetchGroups(repo, browser, cache); err != nil {
		t.Fatalf("could not fetch groups: %v", err)
	}

	// Test.
	type testCase struct {
		name string // description of this test case
		// Named input parameters for target function.
		startTime time.Time
	}
	tests := []testCase{}
	for _, timeStr := range Times {
		startTime, err := time.Parse("15:04", timeStr)
		if err != nil {
			t.Fatalf("could not parse time: %v", err)
		}

		tests = append(tests, testCase{
			name:      "must send notification for the at " + timeStr,
			startTime: startTime,
		})
	}
	for _, tt := range tests {
		if chats, err := repo.GetChats(); err == nil {
			log.Debug().Any("chats", chats).Send()
		} else {
			t.Fatalf("could not get chats: %v", err)
		}

		t.Run(tt.name, func(t *testing.T) {
			b.processPairSending(tt.startTime)
		})
	}

	// Wait for deleting the messages.
	time.Sleep(10 * time.Second)

	utils.Cleanup(t, testsDir)
}

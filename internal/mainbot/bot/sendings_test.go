package bot

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/logger"
	"github.com/rs/zerolog/log"
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
	cfg, repo, browser, cache := initServices(t)
	b, err := New(cfg, nil, repo, browser, cache)
	if err != nil {
		t.Fatalf("could not construct receiver type: %v", err)
	}

	// Test
	groupName := "ИСПт-22-(9)-2"
	now := time.Now()
	timeStr := now.Format("15:04")
	tests := []struct {
		name string // description of this test case

		chats []database.Chat
	}{
		{"send daily schedule", []database.Chat{
			{TgChatID: cfg.Telegram.AdminID, GroupName: &groupName, DailySendingTime: &timeStr},
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

	cleanup(t)
}

func TestBot_processPairSending(t *testing.T) {
	var Times = []string{"7:45", "9:30", "11:15", "13:30", "15:15", "17:00", "18:45"}

	// Initialize services.
	cfg, repo, browser, cache := initServices(t)
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
}

func initServices(t *testing.T) (
	*config.MainConfig,
	*database.Repository,
	*browser.BrowserService,
	*cache.Cache,
) {
	cfg, err := config.LoadMainConfig(debugConfigFile)
	if err != nil {
		t.Fatalf("could not load config: %v", err)
	}
	cfg.Database.File = filepath.Join(testsDir, "database/test.sqlite3")
	cfg.Browser.ScreenshotDir = filepath.Join(testsDir, "storage/screenshots")
	cfg.Cache.Dir = filepath.Join(testsDir, "storage/cache")
	cfg.Logger.Dir = filepath.Join(testsDir, "storage/logs")
	cfg.ScheduleTemplate = templateFile
	logger.SetupLogger(config.LoggerConfig{Level: "trace", Dir: ""})

	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("could not ensure dirs: %v", err)
	}

	// commandsCfg, err := config.LoadCommandsConfig(commandsConfigFile)
	// if err != nil {
	// 	t.Fatalf("could not load config: %v", err)
	// }

	repo, err := database.New(cfg)
	if err != nil {
		t.Fatalf("could not construct repository: %v", err)
	}

	browserService, err := browser.New(cfg)
	if err != nil {
		t.Fatalf("could not construct browser: %v", err)
	}

	cacheService := cache.New(&cfg.Cache)

	return cfg, repo, browserService, cacheService
}

// cleanup removes all files and directories created during testing.
func cleanup(t *testing.T) {
	if err := os.RemoveAll(testsDir); err != nil {
		t.Logf("could not delete tests dir: %v", err)
	}
}

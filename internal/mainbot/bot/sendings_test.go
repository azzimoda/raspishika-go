package bot

import (
	"testing"
	"time"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/logger"
)

func TestBot_processPairSending(t *testing.T) {
	var Times = []string{"7:45", "9:30", "11:15", "13:30", "15:15", "17:00", "18:45"}

	logger.SetupLogger("trace")
	cfg, repo, browserService, cacheService := initServices(t)
	makeTestChats(t, repo, cfg.Telegram.AdminID)
	if _, err := scraper.FetchGroups(repo, browserService, &cacheService); err != nil {
		t.Fatalf("could not fetch groups: %v", err)
	}

	type test struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		cfg     *config.Config
		repo    *database.Repository
		browser *browser.BrowserService
		cache   *cache.Cache
		// Named input parameters for target function.
		startTime time.Time
	}
	tests := []test{}

	for _, timeStr := range Times {
		startTime, err := time.Parse("15:04", timeStr)
		if err != nil {
			t.Fatalf("could not parse time: %v", err)
		}

		tests = append(tests, test{
			name:      "must send notification for the at " + timeStr,
			cfg:       cfg,
			repo:      repo,
			browser:   browserService,
			cache:     &cacheService,
			startTime: startTime,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(tt.cfg, tt.repo, tt.browser, tt.cache)
			if err != nil {
				t.Fatalf("could not construct receiver type: %v", err)
			}
			b.processPairSending(tt.startTime)
		})
	}
}

func initServices(t *testing.T) (*config.Config, *database.Repository, *browser.BrowserService, cache.Cache) {
	cfg, err := config.Load("/home/mazza/code/raspishika-go/configs/config.yml")
	if err != nil {
		t.Fatalf("could not load config: %v", err)
	}

	cfg.Database.File = "/home/mazza/code/raspishika-go/database/test.sqlite3"
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

func makeTestChats(t *testing.T, repo *database.Repository, adminID int64) {
	groupName := "ИСПт-22-(9)-2"
	departmentName := "Отделение СОНХ"

	// Chat with enabled daily and pair sendings.
	chat, err := repo.CreateOrUpdateChat(adminID, "")
	if err != nil {
		t.Fatalf("could not create or update chat: %v", err)
	}

	chat.GroupName = &groupName
	chat.DepartmentName = &departmentName
	chat.DailySendingTime = time.Now().Add(time.Minute).Format("15:04")
	chat.PairSending = true
	if err := repo.UpdateChat(chat); err != nil {
		t.Fatalf("could not update chat: %v", err)
	}

	// Chat with wrong chat ID.
	_, err = repo.CreateOrUpdateChat(0, "")
	if err != nil {
		t.Fatalf("could not create or update chat: %v", err)
	}
}

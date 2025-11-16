package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	adminbot "github.com/azzimoda/raspishika-go/internal/adminbot/bot"
	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	mainbot "github.com/azzimoda/raspishika-go/internal/mainbot/bot"
	"github.com/azzimoda/raspishika-go/internal/mainbot/sendings"

	"github.com/rs/zerolog/log"
)

type App struct {
	Config   *config.MainConfig
	MainBot  *mainbot.Bot
	AdminBot *adminbot.AdminBot
	Repo     *database.Repository
	Browser  *browser.BrowserService
	Cache    *cache.Cache
}

func (a *App) Run() error {
	startTime := time.Now()
	log.Info().Time("start", startTime).Msg("Starting application...")
	defer func() {
		endTime := time.Now()
		log.Info().Time("end", endTime).TimeDiff("duration", startTime, endTime).Msg("Application stopped")
	}()

	go a.MainBot.Start()

	if a.AdminBot != nil {
		go a.AdminBot.Start()
		a.Report().Send("Starting application...")
	}

	sendingManager := sendings.NewSendingManager(a.MainBot.CommandHandler)

	if a.Config.Features.DailySending {
		if err := sendingManager.ScheduleDailySending(); err != nil {
			log.Error().Err(err).Msg("Failed to schedule daily sending, skipping")
		} else {
			log.Info().Msg("Daily sending scheduled")
		}
	}

	if a.Config.Features.PairSending {
		if err := sendingManager.SchedulePairSending(); err != nil {
			log.Error().Err(err).Msg("Failed to schedule pair sending, skipping")
		} else {
			log.Info().Msg("Pair sending scheduled")
		}
	}

	sendingManager.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	a.Shutdown()

	return nil
}

func (a *App) Shutdown() {
	log.Info().Msg("Shutting down application...")
	a.Report().Send("Shutting down application...")

	a.MainBot.Stop()
	log.Info().Msg("Main bot stopped")

	if a.AdminBot != nil {
		a.AdminBot.Stop()
		log.Info().Msg("Admin bot stopped")
	}

	if err := a.Repo.Close(); err != nil {
		log.Error().Err(err).Msg("Database repository closed with error")
	} else {
		log.Info().Msg("Repository closed")
	}

	a.Browser.Close()
}

func (a *App) Report() reporter.ReportConfig {
	if a.AdminBot == nil {
		log.Warn().Msg("Admin bot is not initialized")
		return reporter.ReportConfig{}
	}

	return reporter.NewReportConfig(a.AdminBot.API(), a.Config.Telegram.AdminID)
}

func New(cfg *config.MainConfig, commandsCfg *config.CommandsConfig) (*App, error) {
	app := App{Config: cfg, Cache: cache.New(&cfg.Cache)}
	var err error

	app.Repo, err = database.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	} else {
		log.Debug().Msg("Created repository")
	}

	app.Browser, err = browser.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create browser service: %w", err)
	} else {
		log.Debug().Msg("Created browser service")
	}

	app.MainBot, err = mainbot.New(cfg, commandsCfg.MainBot, app.Repo, app.Browser, app.Cache)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize main bot")
	} else {
		log.Info().Msg("Initialized main bot")
	}
	app.MainBot.Reporter = &app

	if cfg.Features.AdminBot {
		app.AdminBot, err = adminbot.New(cfg, commandsCfg.AdminBot, app.Repo)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize admin bot")
		} else {
			log.Info().Msg("Initialized admin bot")
		}
		app.AdminBot.Reporter = &app
	}

	return &app, nil
}

package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	adminbot "github.com/azzimoda/raspishika-go/internal/adminbot/bot"
	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	mainbot "github.com/azzimoda/raspishika-go/internal/mainbot/bot"

	"github.com/rs/zerolog/log"
)

type App struct {
	Config   *config.Config
	MainBot  *mainbot.Bot
	AdminBot *adminbot.AdminBot
	Repo     *database.Repository
	Browser  *browser.BrowserService
	Cache    *cache.Cache
}

func (a *App) Run() error {
	log.Debug().Msg("Starting application...")

	go a.MainBot.Start()

	if a.AdminBot != nil {
		go a.AdminBot.Start()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	a.Shutdown()
	os.Exit(0)

	return nil
}

func (a *App) Shutdown() {
	log.Info().Msg("Shutting down application...")
	a.Report().Send(`Shutting down application\.\.\.`)

	a.MainBot.Stop()
	log.Debug().Msg("Main bot stopped")

	if a.AdminBot != nil {
		a.AdminBot.Stop()
		log.Debug().Msg("Admin bot stopped")
	}

	if err := a.Repo.Close(); err != nil {
		log.Error().Err(err).Msg("Database repository closed with error")
	} else {
		log.Debug().Msg("Repository closed")
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

func New(cfg *config.Config) (*App, error) {
	repo, err := database.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	} else {
		log.Debug().Msg("Created repository")
	}

	browser, err := browser.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create browser service")
	} else {
		log.Debug().Msg("Created browser service")
	}

	cache := cache.New(&cfg.Cache)

	mainBot, err := mainbot.New(cfg, repo, browser, &cache)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize main bot")
	} else {
		log.Info().Msg("Initialized main bot")
	}

	app := App{
		Config:  cfg,
		MainBot: mainBot,
		Repo:    repo,
		Browser: browser,
		Cache:   &cache,
	}

	mainBot.Reporter = &app

	if cfg.Features.AdminBot {
		app.AdminBot, err = adminbot.New(cfg, repo)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize admin bot")
		}
		app.AdminBot.Reporter = &app
	}

	return &app, nil
}

package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	adminbot "github.com/azzimoda/raspishika-go/internal/adminbot/bot"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	mainbot "github.com/azzimoda/raspishika-go/internal/mainbot/bot"

	"github.com/patrickmn/go-cache"
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
	go a.MainBot.Start()

	if a.AdminBot != nil {
		go a.AdminBot.Start()
	}

	if a.AdminBot != nil {
		go a.AdminBot.Start()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		a.Shutdown()
		os.Exit(0)
	}()

	return nil
}

func (a *App) Shutdown() {
	log.Info().Msg("Shutting down application...")

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

func New(cfg *config.Config) (*App, error) {
	repo, err := database.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %v", err)
	} else {
		log.Debug().Msg("Created repository")
	}

	browser, err := browser.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create browser service")
	} else {
		log.Debug().Msg("Created browser service")
	}

	ttl := time.Duration(cfg.Cache.DefaultTTL) * time.Minute
	cache := cache.New(ttl, ttl*2)

	mainBot, err := mainbot.New(cfg, repo, browser, cache)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize main bot")
	} else {
		log.Debug().Msg("Initialized main bot")
	}

	app := App{
		Config:  cfg,
		MainBot: mainBot,
		Repo:    repo,
		Browser: browser,
		Cache:   cache,
	}

	if cfg.Features.AdminBot {
		app.AdminBot, err = adminbot.New(cfg, repo)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize admin bot")
		}
	}

	return &app, nil
}

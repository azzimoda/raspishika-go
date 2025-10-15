package app

import (
	"fmt"
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

func (a *App) Start() error {
	go a.MainBot.Start()

	if a.AdminBot != nil {
		go a.AdminBot.Start()
	}

	if a.Config.Features.AdminBot {
		adminBot, err := adminbot.New(a.Config, a.Repo)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create admin bot")
		}

		go adminBot.Start()
	}

	return nil
}

func New(cfg *config.Config) (*App, error) {
	repo, err := database.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %v", err)
	}

	browser, err := browser.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create browser")
	}

	ttl := time.Duration(cfg.Cache.DefaultTTL) * time.Minute
	cache := cache.New(ttl, ttl*2)

	mainBot, err := mainbot.New(cfg, repo, browser, cache)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create main bot")
	}

	app := App{
		Config:  cfg,
		MainBot: mainBot,
		Repo:    repo,
		Browser: browser,
		Cache:   cache,
	}

	return &app, nil
}

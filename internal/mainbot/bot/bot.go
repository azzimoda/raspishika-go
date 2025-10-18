package bot

import (
	"errors"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

type Bot struct {
	Config  *config.Config
	api     *tgbotapi.BotAPI
	Repo    *database.Repository
	Browser *browser.BrowserService
	Cache   *cache.Cache
}

func (b *Bot) API() *tgbotapi.BotAPI {
	return b.api
}

func (b *Bot) MyCommands() []map[string]string {
	return b.Config.Telegram.MyCommands
}

func (b *Bot) Start() {
	log.Debug().Msg("Starting main bot...")
	tgbot.StartPolling(b)
}

func (b *Bot) Stop() {
	log.Debug().Msg("Stopping bot...")
	b.api.StopReceivingUpdates()
}

func New(
	cfg *config.Config, repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
) (*Bot, error) {
	log.Debug().Msgf("Token: %s", cfg.Telegram.Token)

	var api *tgbotapi.BotAPI
	err := errors.New("fake error")
	retries := 0
	for retries <= 5 && err != nil {
		api, err = tgbotapi.NewBotAPI(cfg.Telegram.Token)
		if err == nil {
			break
		}
		retries += 1
		log.Error().Err(err).Int("retries", retries).Msg("Failed to connect to Telegram API; retrying...")
	}
	if err != nil {
		return nil, err
	}

	return &Bot{
		Config:  cfg,
		api:     api,
		Repo:    repo,
		Browser: browser,
		Cache:   cache,
	}, nil
}

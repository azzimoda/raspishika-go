package bot

import (
	"errors"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/callbacks"
	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"
	"github.com/azzimoda/raspishika-go/pkg/tgbot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

type Bot struct {
	config          *config.Config
	api             *tgbotapi.BotAPI
	repo            *database.Repository
	browser         *browser.BrowserService
	cache           *cache.Cache
	CommandHandler  *commands.CommandHandler
	CallbackHandler *callbacks.CallbackHandler
	Reporter        reporter.Reporter
}

func (b *Bot) Config() *config.Config {
	return b.config
}

func (b *Bot) API() *tgbotapi.BotAPI {
	return b.api
}

func (b *Bot) Repo() *database.Repository {
	return b.repo
}

func (b *Bot) Browser() *browser.BrowserService {
	return b.browser
}

func (b *Bot) Cache() *cache.Cache {
	return b.cache
}

func (b *Bot) Start() {
	log.Info().Msg("Starting main bot...")
	tgbot.SetMyCommands(b.api, b.config.Telegram.MyCommands)
	tgbot.StartPolling(b)
}

func (b *Bot) Stop() {
	log.Info().Msg("Stopping bot...")
	b.api.StopReceivingUpdates()
}

func (b *Bot) Report() reporter.ReportConfig {
	if b.Reporter == nil {
		return reporter.ReportConfig{}
	}
	return b.Reporter.Report()
}

func New(
	cfg *config.Config,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
) (*Bot, error) {
	log.Debug().Msgf("Token: %s", cfg.Telegram.Token)

	bot := Bot{
		config:  cfg,
		api:     nil,
		repo:    repo,
		browser: browser,
		cache:   cache,
	}
	bot.CommandHandler = &commands.CommandHandler{Bot: &bot}
	bot.CallbackHandler = &callbacks.CallbackHandler{Bot: &bot}

	err := errors.New("fake error")
	retries := 0
	for retries <= 5 && err != nil {
		bot.api, err = tgbotapi.NewBotAPI(cfg.Telegram.Token)
		if err == nil {
			break
		}
		retries += 1
		log.Error().Err(err).Int("retries", retries).Msg("Failed to connect to Telegram API; retrying...")
	}
	if err != nil {
		return nil, err
	}

	return &bot, nil
}

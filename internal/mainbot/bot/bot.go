package bot

import (
	"errors"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/callbacks"
	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/internal/services"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
)

type Bot struct {
	config          *config.MainConfig
	myCommands      []map[string]string
	bot             *tgbotapi.BotAPI
	services        *services.Services
	CommandHandler  *commands.CommandHandler
	CallbackHandler *callbacks.CallbackHandler
}

func (b *Bot) Config() *config.MainConfig {
	return b.config
}

func (b *Bot) API() *tgbotapi.BotAPI {
	return b.bot
}

func (b *Bot) Repo() *database.Repository {
	return b.services.Repo
}

func (b *Bot) Browser() *browser.BrowserService {
	return b.services.Browser
}

func (b *Bot) Cache() *cache.Cache {
	return b.services.Cache
}

func (b *Bot) ScheduleManager() *scraper.ScheduleManager {
	return b.services.ScheduleManager
}

func (b *Bot) Start() {
	tgbothelpers.SetMyCommandsOld(b.bot, b.myCommands)
	tgbothelpers.StartPolling(b)
	log.Info().Msg("Main bot started")
}

func (b *Bot) Stop() {
	b.bot.StopReceivingUpdates()
	log.Info().Msg("Bot stopped")
}

func (b *Bot) Report() reporter.ReportConfig {
	if b.services.Reporter == nil {
		return reporter.ReportConfig{}
	}
	return b.services.Reporter.Report()
}

func New(
	cfg *config.MainConfig,
	myCommands []map[string]string,
	services *services.Services,
) (*Bot, error) {
	bot := Bot{config: cfg, myCommands: myCommands, services: services}
	bot.CommandHandler = &commands.CommandHandler{Bot: &bot}
	bot.CallbackHandler = &callbacks.CallbackHandler{Bot: &bot}

	err := errors.New("fake error")
	retries := 0
	for retries <= 5 && err != nil {
		bot.bot, err = tgbotapi.NewBotAPI(cfg.Telegram.Token)
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

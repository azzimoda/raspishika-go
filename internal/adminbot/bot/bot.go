package bot

import (
	"errors"
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbot"
	"github.com/azzimoda/raspishika-go/pkg/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

type AdminBot struct {
	Config      *config.MainConfig
	myCommands  []map[string]string
	adminConfig *config.AdminConfig
	api         *tgbotapi.BotAPI
	repo        *database.Repository
	Reporter    reporter.Reporter
}

func (b *AdminBot) API() *tgbotapi.BotAPI {
	return b.api
}

func (b *AdminBot) Start() {
	log.Info().Msg("Starting admin bot...")
	tgbot.SetMyCommands(b.api, b.myCommands)
	tgbot.StartPolling(b)
}

func (b *AdminBot) Stop() {
	b.api.StopReceivingUpdates()
}

func (b *AdminBot) Report() reporter.ReportConfig {
	if b.Reporter == nil {
		return reporter.ReportConfig{}
	}
	return b.Reporter.Report().Admin()
}

func (b *AdminBot) ReportNewChat(chat *database.Chat) {
	if !b.adminConfig.NewChatReport {
		log.Trace().Msg("New chat report is disabled.")
		return
	}
	b.Report().Chat(chat.TgChatID).
		Sendf("Registered new chat with group %s.", utils.DerefOrTypeDefault(chat.GroupName))
}

func New(cfg *config.MainConfig, myCommands []map[string]string, repo *database.Repository) (*AdminBot, error) {
	adminCfg, err := config.LoadAdminConfig(cfg.AdminConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin config: %w", err)
	}

	bot := AdminBot{Config: cfg, myCommands: myCommands, adminConfig: adminCfg, repo: repo}

	err = errors.New("fake error")
	retries := 0
	for retries <= 5 && err != nil {
		bot.api, err = tgbotapi.NewBotAPI(cfg.Telegram.AdminToken)
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

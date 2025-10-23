package bot

import (
	"errors"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

type AdminBot struct {
	Config   *config.Config
	api      *tgbotapi.BotAPI
	Repo     *database.Repository
	Reporter reporter.Reporter
}

func (b *AdminBot) API() *tgbotapi.BotAPI {
	return b.api
}

func (b *AdminBot) Start() {
	log.Info().Msg("Starting admin bot...")
	tgbot.StartPolling(b)
}

func (b *AdminBot) Stop() {
	b.api.StopReceivingUpdates()
}

func New(cfg *config.Config, repo *database.Repository) (*AdminBot, error) {
	log.Debug().Msgf("Admin token: %s", cfg.Telegram.AdminToken)

	var api *tgbotapi.BotAPI
	err := errors.New("fake error")
	retries := 0
	for retries <= 5 && err != nil {
		api, err = tgbotapi.NewBotAPI(cfg.Telegram.AdminToken)
		if err == nil {
			break
		}
		retries += 1
		log.Error().Err(err).Int("retries", retries).Msg("Failed to connect to Telegram API; retrying...")
	}
	if err != nil {
		return nil, err
	}

	return &AdminBot{
		Config: cfg,
		api:    api,
		Repo:   repo,
	}, nil
}

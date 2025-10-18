package bot

import (
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

type AdminBot struct {
	Config *config.Config
	api    *tgbotapi.BotAPI
	Repo   *database.Repository
}

func (b *AdminBot) API() *tgbotapi.BotAPI {
	return b.api
}

func (b *AdminBot) Start() {
	log.Debug().Msg("Starting main bot...")
	tgbot.StartPolling(b)
}

func (b *AdminBot) Stop() {
	b.api.StopReceivingUpdates()
}

func New(cfg *config.Config, repo *database.Repository) (*AdminBot, error) {
	panic("unimplemented: adminbot.AdminBot")
}

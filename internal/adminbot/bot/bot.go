package bot

import (
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type AdminBot struct {
	Config *config.Config
	api    tgbotapi.BotAPI
	Repo   *database.Repository
}

func (b *AdminBot) Start() {
	panic("unimplemented: adminbot.AdminBot.Start()")
}

func (b *AdminBot) Stop() {
	b.api.StopReceivingUpdates()
}

func New(cfg *config.Config, repo *database.Repository) (*AdminBot, error) {
	panic("unimplemented: adminbot.AdminBot")
}

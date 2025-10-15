package bot

import (
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/patrickmn/go-cache"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	panic("unimplemented")
}

func New(
	cfg *config.Config, repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
) (*Bot, error) {
	panic("unimplemented")
}

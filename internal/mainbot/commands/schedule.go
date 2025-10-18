package commands

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func OnWeek(
	api *tgbotapi.BotAPI, repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
	msg *tgbotapi.Message,
) error {
	return fmt.Errorf("Unimplemented: commands.OnWeek")
}

func OnTomorrow(api *tgbotapi.BotAPI, repo *database.Repository, cache *cache.Cache, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnTomorrow")
}

func OnLeft(api *tgbotapi.BotAPI, repo *database.Repository, cache *cache.Cache, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnLeft")
}

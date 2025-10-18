package commands

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// OnQuick sends department selection menu.
func OnQuick(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: OnQuick")
}

func OnTeacher(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: OnTeacher")
}

package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/rs/zerolog/log"
)

func (b *Bot) ApplyMiddleware(update tgbotapi.Update) bool {
	log.Error().Msg("Unimplemented: Bot.ApplyMiddleWare")

	return true
}

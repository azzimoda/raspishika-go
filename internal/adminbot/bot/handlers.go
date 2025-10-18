package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (b *AdminBot) OnUpdate(update tgbotapi.Update) {
	log.Error().Msg("Unimplemented: AdminBot.OnUpdate")
}

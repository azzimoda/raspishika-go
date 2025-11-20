package mainbot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
)

func (mb *MainBot) reminderOnHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Reminder on handler")
	// TODO: Implement.
}

func (mb *MainBot) reminderOffHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Reminder off handler")
	// TODO: Implement.
}

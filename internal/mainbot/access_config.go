package mainbot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
)

func (mb *MainBot) accessHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Access handler")
	// TODO: Implement.
}

func (mb *MainBot) setAccessHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Set access handler")
	// TODO: Implement.
}

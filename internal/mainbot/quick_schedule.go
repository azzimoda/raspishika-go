package mainbot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
)

func (mb *MainBot) quickHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Quick handler")
	// TODO: Implement.
}

func (mb *MainBot) quickSelectDepartmentHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Quick select department handler")
	// TODO: Implement.
}

func (mb *MainBot) textQuickGroupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Quick select course handler")
	// TODO: Implement.
}

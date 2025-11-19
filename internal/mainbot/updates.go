package mainbot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
)

func (mb *MainBot) updateGroupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Update group handler")
	// TODO: Implement.
}

func (mb *MainBot) updateTeacherHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Update teacher handler")
	// TODO: Implement.
}

func (mb *MainBot) updateTomorrowHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Update tomorrow handler")
	// TODO: Implement.
}

func (mb *MainBot) updateLeftHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Update left handler")
	// TODO: Implement.
}

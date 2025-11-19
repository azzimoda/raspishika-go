package mainbot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
)

func (mb *MainBot) teacherHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Teacher handler")
	// TODO: Implement.
}

// TODO: Split original logic into two smaller functions.
func (mb *MainBot) selectTeacherHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Select teacher handler")
	// TODO: Implement.
}

func (mb *MainBot) textTeacherNameHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Text teacher name handler")
	// TODO: Implement.
}

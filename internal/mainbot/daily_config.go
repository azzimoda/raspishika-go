package mainbot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
)

func (mb *MainBot) dailyTimeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Daily time handler")
	// TODO: Implement.
}

func (mb *MainBot) dailyOffHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Daily off handler")
	// TODO: Implement.
}

func (mb *MainBot) textTimeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Text time handler")
	// TODO: Implement.
}

func (mb *MainBot) configDailyTimeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Config daily time handler")
	// TODO: Implement.
}

func (mb *MainBot) dailyOffCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Daily off callback handler")
	// TODO: Implement.
}

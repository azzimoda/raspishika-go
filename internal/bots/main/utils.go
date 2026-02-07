package mainbot

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

// addContextHandlerError adds an error to the handler error context.
func addContextHandlerError(ctx context.Context, err error) {
	handlerErrs, ok := ctx.Value(errorContextKey).(*[]error)
	if ok {
		if err != nil {
			*handlerErrs = append(*handlerErrs, err)
		}
	} else {
		log.Warn().Err(err).Msg("Error context not found")
	}
}

// mainMenuReplyMarkup returns the main menu keyboard for the given chat type.
func mainMenuReplyMarkup(isPrivate bool) models.ReplyMarkup {
	if isPrivate {
		return models.ReplyKeyboardMarkup{
			Keyboard: [][]models.KeyboardButton{
				{{Text: "Неделя"}},
				{{Text: "Сегодня"}, {Text: "Завтра"}, {Text: "Преподаватель"}},
			},
			ResizeKeyboard: true,
		}
	} else {
		return models.ReplyKeyboardRemove{RemoveKeyboard: true}
	}
}

func sendErrorMessage(ctx context.Context, b *bot.Bot, params *bot.SendMessageParams) error {
	err := bothelpers.SendTempMessage(ctx, b, 7*time.Second, params)
	if err != nil {
		log.Error().Err(err).Any("params", params).Msg("Failed to send error message")
	}
	return err
}

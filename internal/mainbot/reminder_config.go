package mainbot

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
)

func (mb *MainBot) reminderOnHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Reminder on handler")
	mb.setReminderHelper(ctx, b, update, true)
}

func (mb *MainBot) reminderOffHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Reminder off handler")
	mb.setReminderHelper(ctx, b, update, false)
}

func (mb *MainBot) setReminderHelper(ctx context.Context, b *bot.Bot, update *models.Update, on bool) {
	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
	})
	addContextHandlerError(ctx, err)

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	chat.PairSending = on
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgCouldNotUpdateData})
		return
	}

	text := "Напоминания выключены"
	if on {
		text = "Напоминания включены"
	}
	err = tgbothelpers.SendTempMessage(ctx, b, 5*time.Second, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	})
	addContextHandlerError(ctx, err)
}

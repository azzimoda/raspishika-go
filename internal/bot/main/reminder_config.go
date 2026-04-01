package mainbot

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/botutil"
	"github.com/azzimoda/raspishika-go/internal/model"
	bothelpers "github.com/azzimoda/raspishika-go/pkg/bothelper"
)

func (mb *MainBot) reminderOnHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Reminder on handler")
	mb.setReminderHelper(ctx, b, update, true)
}

func (mb *MainBot) reminderOffHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Reminder off handler")
	mb.setReminderHelper(ctx, b, update, false)
}

func (mb *MainBot) setReminderHelper(ctx context.Context, b *bot.Bot, update *tgmodels.Update, on bool) {
	_, err := bothelpers.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		return
	}

	chat.PairSending = on
	if err := mb.container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}

	text := "Напоминания перед парами выключены"
	if on {
		text = "Напоминания перед парами включены"
	}
	err = bothelpers.SendTempMessage(ctx, b, 5*time.Second, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
	})
	addContextHandlerError(ctx, err)
}

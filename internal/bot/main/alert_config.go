package mainbot

import (
	"context"
	"time"

	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/pkg/bothelper"
	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

func (mb *MainBot) alertOnHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	mb.setChangeAlert(ctx, b, update, true)
}

func (mb *MainBot) alertOffHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	mb.setChangeAlert(ctx, b, update, false)
}

func (mb *MainBot) setChangeAlert(ctx context.Context, b *bot.Bot, update *tgmodels.Update, on bool) {
	_, err := bothelper.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}

	chat.ChangeAlert = on
	if err := mb.services.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}

	text := "Уведомления об изменениях выключены"
	if on {
		text = "Уведомления об изменениях включены"
	}
	err = bothelper.SendTempMessage(ctx, b, 5*time.Second, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
	})
	addContextHandlerError(ctx, err)
}

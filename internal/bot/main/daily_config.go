package mainbot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/botutil"
	"github.com/azzimoda/raspishika-go/internal/model"
	bothelpers "github.com/azzimoda/raspishika-go/pkg/bothelper"
)

// Commands

func (mb *MainBot) dailyTimeHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Daily time handler")

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

	if err := mb.container.Chat.Update(chat.WithState(model.ChatStateSelectingTime)); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}

	time := "<i>время не установлено</i>"
	if chat.DailySendingTime != nil {
		time = "Установленное время: <i>" + *chat.DailySendingTime + "</i>"
	}
	text := fmt.Sprintf("%s\nПришлите желаемое время рассылки, например <code>19:00</code>", time)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
		ParseMode:       tgmodels.ParseModeHTML,
		ReplyMarkup: tgmodels.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
				{{Text: "Закрыть", CallbackData: CallbackCommandDeleteConfig}},
			},
		},
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) dailyOffHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Daily off handler")

	chat, err := mb.container.Chat.GetByChatID(model.ChatID(update.Message.Chat.ID))
	if err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		return
	}

	chat.DailySendingTime = nil
	if err := mb.container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Ежедневная рассылка выключена",
	})
	addContextHandlerError(ctx, err)
}

// Text messages

func (mb *MainBot) textTimeHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Text time handler")

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

	t, err := time.Parse("15:04", update.Message.Text)
	if err != nil {
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Неправильный вормат времени, попробуйте ещё раз: <code>19:00</code>",
			ParseMode:       tgmodels.ParseModeHTML,
		})
		return
	}
	timeStr := t.Format("15:04")

	chat.DailySendingTime = &timeStr
	if err := mb.container.Chat.Update(chat.WithState(model.ChatStateDefault)); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Время рассылки установлено на " + timeStr,
	})
	addContextHandlerError(ctx, err)
}

// Callback queries

func (mb *MainBot) configDailyTimeHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config daily time handler")

	message := update.CallbackQuery.Message.Message
	_, err := bothelpers.DeleteMessageSafely(ctx, b, message)
	addContextHandlerError(ctx, err)

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		return
	}

	if err := mb.container.Chat.Update(chat.WithState(model.ChatStateSelectingTime)); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}
	time := "<i>Время не установлено</i>"
	if chat.DailySendingTime != nil {
		time = "Установленное время: <i>" + *chat.DailySendingTime + "</i>"
	}
	text := fmt.Sprintf("%s\nПришлите желаемое время рассылки, например <code>19:00</code>", time)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          message.Chat.ID,
		MessageThreadID: message.MessageThreadID,
		Text:            text,
		ParseMode:       tgmodels.ParseModeHTML,
		ReplyMarkup: tgmodels.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
				{{Text: "Закрыть", CallbackData: CallbackCommandDelete}},
			},
		},
	})
	addContextHandlerError(ctx, err)
}

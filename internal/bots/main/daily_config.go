package mainbot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

// Commands

func (mb *MainBot) dailyTimeHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Daily time handler")

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}

	if err := models.UpdateChatState(mb.services.Repository.DB, chat.TgChatID, models.ChatStateSelectingTime); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
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

	chat, err := models.GetChatByTgChatID(mb.services.Repository.DB, models.ChatID(update.Message.Chat.ID))
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}

	chat.DailySendingTime = nil
	if err := chat.Update(mb.services.Repository.DB); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
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

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}

	t, err := time.Parse("15:04", update.Message.Text)
	if err != nil {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Неправильный вормат времени, попробуйте ещё раз: <code>19:00</code>",
			ParseMode:       tgmodels.ParseModeHTML,
		})
		return
	}
	timeStr := t.Format("15:04")

	chat.State = models.ChatStateDefault
	chat.DailySendingTime = &timeStr
	if err := chat.Update(mb.services.Repository.DB); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
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

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}
	if err := models.UpdateChatState(mb.services.Repository.DB, chat.TgChatID, models.ChatStateSelectingTime); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
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

package mainbot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

// Commands

func (mb *MainBot) dailyTimeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Daily time handler")

	chat, ok := ctx.Value(chatContextKey).(*repository.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}

	if err := mb.services.Repo.UpdateChatState(chat.TgChatID, repository.ChatStateSelectingTime); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}

	time := ""
	if chat.DailySendingTime == nil {
		time = "Время не установлено"
	} else {
		time = "Установленное время: " + *chat.DailySendingTime
	}
	text := fmt.Sprintf("_%s_\nПришлите желаемое время рассылки, например `19:00`", time)

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
		ParseMode:       models.ParseModeMarkdown,
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "Закрыть", CallbackData: "delete_config"}}},
		},
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) dailyOffHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Daily off handler")

	chat, err := mb.services.Repo.GetChatByTgChatID(update.Message.Chat.ID)
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
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
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

func (mb *MainBot) textTimeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Text time handler")

	chat, ok := ctx.Value(chatContextKey).(*repository.Chat)
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
			Text:            "Неправильный вормат времени, попробуйте ещё раз: `19:00`",
			ParseMode:       models.ParseModeMarkdown,
		})
		return
	}
	timeStr := t.Format("15:04")

	chat.State = repository.ChatStateDefault
	chat.DailySendingTime = &timeStr
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
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

func (mb *MainBot) configDailyTimeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Config daily time handler")

	message := update.CallbackQuery.Message.Message
	_, err := bothelpers.DeleteMessageSafely(ctx, b, message)
	addContextHandlerError(ctx, err)

	chat, ok := ctx.Value(chatContextKey).(*repository.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}
	if err := mb.services.Repo.UpdateChatState(chat.TgChatID, repository.ChatStateSelectingTime); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}
	time := ""
	if chat.DailySendingTime == nil {
		time = "Время не установлено"
	} else {
		time = "Установленное время: " + *chat.DailySendingTime
	}
	text := fmt.Sprintf("_%s_\nПришлите желаемое время рассылки, например `19:00`", time)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          message.Chat.ID,
		MessageThreadID: message.MessageThreadID,
		Text:            text,
		ParseMode:       models.ParseModeMarkdown,
		ReplyMarkup:     models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "Закрыть", CallbackData: "delete"}}}},
	})
	addContextHandlerError(ctx, err)
}

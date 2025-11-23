package mainbot

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
)

func (mb *MainBot) accessHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Access handler")

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

	if chat.IsPrivate() {
		tgbothelpers.SendTempMessage(ctx, b, 5*time.Second, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Настройки доступа доступны только в групповых чатах",
		})
	} else {
		text := fmt.Sprintf(
			`Текущий уровень доступа: %d

	0 — без ограничений
	1 — настройки только для админов
	2 — все команды только для админов`,
			chat.Access,
		)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      update.Message.Chat.ID,
			Text:        text,
			ReplyMarkup: accessMenuInlineMarkup(chat.Access),
		})
	}
}

func (mb *MainBot) setAccessHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Set access handler")

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	command := ParseCallbackData(update.CallbackQuery.Data)
	accessLevel, err := strconv.Atoi(command.Args[0])
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Произошла ошибка, установлено значение по умолчанию — 0",
		})
		log.Error().Err(err).Msg("Failed to parse access level; fallback to 0")
		chat.Access = 0
	} else {
		chat.Access = database.ChatAccessLevel(accessLevel)
	}

	if err := mb.services.Repo.UpdateChat(chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgCouldNotUpdateData})
		return
	}

	text := fmt.Sprintf(
		`Текущий уровень доступа: %d

	0 — без ограничений
	1 — настройки только для админов
	2 — все команды только для админов`,
		chat.Access,
	)
	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        text,
		ReplyMarkup: accessMenuInlineMarkup(chat.Access),
	})
	addContextHandlerError(ctx, err)
}

func accessMenuInlineMarkup(accessLevel database.ChatAccessLevel) *models.InlineKeyboardMarkup {
	keyboard := [][]models.InlineKeyboardButton{
		{},
		{{Text: "Закрыть", CallbackData: "delete"}},
	}
	for i := range 3 {
		text := fmt.Sprint(i)
		if i == int(accessLevel) {
			text = fmt.Sprintf("[%d]", i)
		}
		keyboard[0] = append(keyboard[0], models.InlineKeyboardButton{
			Text:         text,
			CallbackData: fmt.Sprintf("set_access\n%d", i),
		})
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

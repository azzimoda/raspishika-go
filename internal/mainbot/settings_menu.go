package mainbot

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (mb *MainBot) settingsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Settings handler")

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	_, err := tgbothelpers.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	text, replyMarkup := settingsMessageParams(chat)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: replyMarkup,
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) configGroupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Config group handler")

	message := update.CallbackQuery.Message.Message
	_, err := tgbothelpers.DeleteMessageSafely(ctx, b, message)
	addContextHandlerError(ctx, err)

	mb.sendGroupMenu(ctx, b)
}

func (mb *MainBot) dailyOffCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Daily off callback handler")
	message := update.CallbackQuery.Message.Message

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	chat.DailySendingTime = nil
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgCouldNotUpdateData})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configReminderHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Config reminder handler")

	command := tgbothelpers.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	chat.PairSending = command.Arg(0) == "true"
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgCouldNotUpdateData})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configAccessHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Config access handler")

	command := tgbothelpers.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	accessLevel, err := strconv.Atoi(command.Arg(0))
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
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgCouldNotUpdateData})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func updateSettingsMenu(ctx context.Context, b *bot.Bot, update *models.Update, chat *database.Chat) {
	log.Trace().Msg("Update settings menu")

	text, markup := settingsMessageParams(chat)
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	addContextHandlerError(ctx, err)
}

func settingsMessageParams(chat *database.Chat) (string, *models.InlineKeyboardMarkup) {
	// Text
	dailyTime := "выключено"
	if chat.DailySendingTime != nil {
		dailyTime = *chat.DailySendingTime
	}
	pairNotification := "выключено"
	if chat.PairSending {
		pairNotification = "включено"
	}

	text := fmt.Sprintf(`<b>Меню настроек</b>

Группа: <u>%s</u>
Ежедневная рассылка: <u>%s</u>
Напоминания перед парами: <u>%s</u>`,
		utils.DerefOrTypeDefault(chat.GroupName), dailyTime, pairNotification)
	if !chat.IsPrivate() {
		text += fmt.Sprintf("\nУровень доступа: <u>%d</u>", chat.Access)
	}

	// Keyboard
	keyboard := make([][]models.InlineKeyboardButton, 0)
	keyboard = append(keyboard, []models.InlineKeyboardButton{{Text: "Изменить группу", CallbackData: "config_group"}})
	if chat.DailySendingTime == nil {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "Включить ежедневную рассылку", CallbackData: "config_daily_time"},
		})
	} else {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "Изменить время", CallbackData: "config_daily_time"},
			{Text: "Выключить рассылку", CallbackData: "daily_off"},
		})
	}
	if chat.PairSending {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "Выключить напоминания", CallbackData: "config_reminder\nfalse"},
		})
	} else {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "Включить напоминания", CallbackData: "config_reminder\ntrue"},
		})
	}
	if !chat.IsPrivate() {
		row := []models.InlineKeyboardButton{
			{Text: "0", CallbackData: "set_access\n0"},
			{Text: "1", CallbackData: "set_access\n1"},
			{Text: "2", CallbackData: "set_access\n2"},
		}
		for i := range 3 {
			if i == int(chat.Access) {
				row[i].Text = fmt.Sprintf("[%d]", chat.Access)
			}
		}
		keyboard = append(keyboard, row)
	}
	keyboard = append(keyboard, []models.InlineKeyboardButton{{Text: "Закрыть", CallbackData: "delete"}})
	// TODO: use `delete_config`.

	return text, &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

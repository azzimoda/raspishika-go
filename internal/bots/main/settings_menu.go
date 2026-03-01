package mainbot

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (mb *MainBot) settingsHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Settings handler")

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

	_, err := bothelpers.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	text, replyMarkup := settingsMessageParams(chat)
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
		ParseMode:       tgmodels.ParseModeHTML,
		ReplyMarkup:     replyMarkup,
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) configGroupHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config group handler")

	message := update.CallbackQuery.Message.Message
	_, err := bothelpers.DeleteMessageSafely(ctx, b, message)
	addContextHandlerError(ctx, err)

	mb.sendGroupMenu(ctx, b, message.MessageThreadID)
}

func (mb *MainBot) dailyOffCallbackHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Daily off callback handler")
	message := update.CallbackQuery.Message.Message

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

	chat.DailySendingTime = nil
	if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configReminderHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config reminder handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

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

	chat.PairSending = command.Arg(0) == "true"
	if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configChangeHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config change handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

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

	chat.ChangeAlert = command.Arg(0) == "true"
	if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configAccessHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config access handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

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

	accessLevel, err := strconv.Atoi(command.Arg(0))
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            "Произошла ошибка, установлено значение по умолчанию — 0",
		})
		log.Error().Err(err).Msg("Failed to parse access level; fallback to 0")
		chat.Access = 0
	} else {
		chat.Access = models.ChatAccessLevel(accessLevel)
	}

	if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func updateSettingsMenu(ctx context.Context, b *bot.Bot, update *tgmodels.Update, chat *models.Chat) {
	log.Trace().Msg("Update settings menu")

	text, markup := settingsMessageParams(chat)
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        text,
		ParseMode:   tgmodels.ParseModeHTML,
		ReplyMarkup: markup,
	})
	addContextHandlerError(ctx, err)
}

func settingsMessageParams(chat *models.Chat) (string, *tgmodels.InlineKeyboardMarkup) {
	text := settingsMessageText(chat)
	keyboard := settingsMessageKeyboard(chat)

	return text, &tgmodels.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func settingsMessageText(chat *models.Chat) string {
	dailyTime := "выключено"
	if chat.DailySendingTime != nil {
		dailyTime = *chat.DailySendingTime
	}

	pairNotification := "выключено"
	if chat.PairSending {
		pairNotification = "включено"
	}

	changesNotificatin := "выключено"
	if chat.ChangeAlert {
		changesNotificatin = "включено"
	}

	// TODO: Use HTML instead of Markdown everywhere.
	// TODO: Use `text/template` here.
	text := fmt.Sprintf(`<b>Меню настроек</b>

Группа: <u>%s</u>
Ежедневная рассылка: <u>%s</u>
Напоминания перед парами: <u>%s</u>
Уведомления об изменениях: <u>%s</u>`,
		utils.DerefOrTypeDefault(chat.GroupName),
		dailyTime,
		pairNotification,
		changesNotificatin,
	)
	if !chat.IsPrivate() {
		text += fmt.Sprintf("\nУровень доступа: <u>%d</u>", chat.Access)
	}

	return text
}

func settingsMessageKeyboard(chat *models.Chat) [][]tgmodels.InlineKeyboardButton {
	keyboard := make([][]tgmodels.InlineKeyboardButton, 0)

	// Student group
	keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{{Text: "Изменить группу", CallbackData: "config_group"}})

	// Daily sending
	if chat.DailySendingTime == nil {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Вкл. ежедневную рассылку", CallbackData: "config_daily_time"},
		})
	} else {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Изменить время", CallbackData: "config_daily_time"},
			{Text: "Выкл. рассылку", CallbackData: "daily_off"},
		})
	}

	// Pair notification
	if chat.PairSending {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Выкл. напоминания пар", CallbackData: "config_reminder\nfalse"},
		})
	} else {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Вкл. напоминания пар", CallbackData: "config_reminder\ntrue"},
		})
	}

	// Changes alerts
	if chat.ChangeAlert {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Выкл. уведомления изменений", CallbackData: "config_change\nfalse"},
		})
	} else {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Вкл. уведомления изменений", CallbackData: "config_change\ntrue"},
		})
	}

	// Group chat access
	if !chat.IsPrivate() {
		row := []tgmodels.InlineKeyboardButton{
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

	// Close button
	keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{{Text: "Закрыть", CallbackData: "delete_config"}})

	return keyboard
}

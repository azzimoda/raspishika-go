package mainbot

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/botutil"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/pkg/bothelper"
	"github.com/azzimoda/raspishika-go/pkg/refutil"
)

func (mb *MainBot) settingsHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Settings handler")

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

	_, err := bothelper.DeleteMessageSafely(ctx, b, update.Message)
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
	_, err := bothelper.DeleteMessageSafely(ctx, b, message)
	addContextHandlerError(ctx, err)

	mb.sendGroupMenu(ctx, b, message.MessageThreadID)
}

func (mb *MainBot) dailyOffCallbackHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Daily off callback handler")
	message := update.CallbackQuery.Message.Message

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

	chat.DailySendingTime = nil
	if err := mb.services.Container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configReminderHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config reminder handler")

	command := bothelper.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

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

	chat.PairSending = command.Arg(0) == "true"
	if err := mb.services.Container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configChangeHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config change handler")

	command := bothelper.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

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

	chat.ChangeAlert = command.Arg(0) == "true"
	if err := mb.services.Container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configDarkModeHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config dark mode handler")

	command := bothelper.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

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

	chat.DarkMode = command.Arg(0) == "true"
	if err := mb.services.Container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func (mb *MainBot) configAccessHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Config access handler")

	command := bothelper.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

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

	accessLevel, err := strconv.Atoi(command.Arg(0))
	if err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            "Произошла ошибка, установлено значение по умолчанию — 0",
		})
		log.Error().Err(err).Msg("Failed to parse access level; fallback to 0")
		chat.Access = 0
	} else {
		chat.Access = model.ChatAccessLevel(accessLevel)
	}

	if err := mb.services.Container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}

	updateSettingsMenu(ctx, b, update, chat)
}

func updateSettingsMenu(ctx context.Context, b *bot.Bot, update *tgmodels.Update, chat *model.Chat) {
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

func settingsMessageParams(chat *model.Chat) (string, *tgmodels.InlineKeyboardMarkup) {
	text := settingsMessageText(chat)
	keyboard := settingsMessageKeyboard(chat)

	return text, &tgmodels.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func settingsMessageText(chat *model.Chat) string {
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

	theme := "светлая"
	if chat.DarkMode {
		theme = "тёмная"
	}

	// TODO: Use `text/template` here.
	text := fmt.Sprintf(`<b>Меню настроек</b>

Группа: <u>%s</u>
Ежедневная рассылка: <u>%s</u>
Напоминания перед парами: <u>%s</u>
Уведомления об изменениях: <u>%s</u>
Тема: <u>%s</u>`,
		refutil.DerefOrTypeDefault(chat.GroupName),
		dailyTime,
		pairNotification,
		changesNotificatin,
		theme,
	)
	if !chat.IsPrivate() {
		text += fmt.Sprintf("\nУровень доступа: <u>%d</u>", chat.Access)
	}

	return text
}

func settingsMessageKeyboard(chat *model.Chat) [][]tgmodels.InlineKeyboardButton {
	keyboard := make([][]tgmodels.InlineKeyboardButton, 0)

	// Student group
	keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{{Text: "Изменить группу", CallbackData: CallbackCommandConfigGroup}})

	// Daily sending
	if chat.DailySendingTime == nil {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Вкл. ежедневную рассылку", CallbackData: CallbackCommandConfigDailyTime},
		})
	} else {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Изменить время", CallbackData: CallbackCommandConfigDailyTime},
			{Text: "Выкл. рассылку", CallbackData: CallbackCommandDailyOff},
		})
	}

	// Pair notification
	if chat.PairSending {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Выкл. напоминания перед парами", CallbackData: CallbackCommandConfigReminder + "\nfalse"},
		})
	} else {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Вкл. напоминания перед парами", CallbackData: CallbackCommandConfigReminder + "\ntrue"},
		})
	}

	// Change alerts
	if chat.ChangeAlert {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Выкл. уведомления об изменениях", CallbackData: CallbackCommandConfigChange + "\nfalse"},
		})
	} else {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Вкл. уведомления об изменениях", CallbackData: CallbackCommandConfigChange + "\ntrue"},
		})
	}

	// Dark mode
	if chat.DarkMode {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Вкл. светлую тему", CallbackData: CallbackCommandConfigDarkMode + "\nfalse"},
		})
	} else {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
			{Text: "Вкл. тёмную тему", CallbackData: CallbackCommandConfigDarkMode + "\ntrue"},
		})
	}

	// Group chat access
	if !chat.IsPrivate() {
		row := []tgmodels.InlineKeyboardButton{
			{Text: "0", CallbackData: CallbackCommandSetAccess + "\n0"},
			{Text: "1", CallbackData: CallbackCommandSetAccess + "\n1"},
			{Text: "2", CallbackData: CallbackCommandSetAccess + "\n2"},
		}
		for i := range 3 {
			if i == int(chat.Access) {
				row[i].Text = fmt.Sprintf("[%d]", chat.Access)
			}
		}
		keyboard = append(keyboard, row)
	}

	// Close button
	keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
		{Text: "Закрыть", CallbackData: CallbackCommandDeleteConfig},
	})

	return keyboard
}

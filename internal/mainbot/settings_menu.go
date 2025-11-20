package mainbot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
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

	text, replyMarkup := settingsMessageParams(chat)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        text,
		ReplyMarkup: replyMarkup,
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) configGroupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Config group handler")
	// TODO: Implement.
}

func (mb *MainBot) configReminderHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Config reminder handler")
	// TODO: Implement.
}

func (mb *MainBot) configAccessHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Config access handler")
	// TODO: Implement.
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

	text := fmt.Sprintf(`Меню настроек

Группа: %s
Ежедневная рассылка: %s
Напоминания перед парами: %s`,
		utils.DerefOrTypeDefault(chat.GroupName), dailyTime, pairNotification)
	if !chat.IsPrivate() {
		text += fmt.Sprintf("\nУровень доступа: %d", chat.Access)
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
			{Text: "Изменить время", CallbackData: "daily_time_config"},
			{Text: "Выключить рассылку", CallbackData: "daily_off"},
		})
	}
	if chat.PairSending {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "Выключить напоминания", CallbackData: "config_reminder"},
		})
	} else {
		keyboard = append(keyboard, []models.InlineKeyboardButton{
			{Text: "Включить напоминания", CallbackData: "config_reminder"},
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

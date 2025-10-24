package utils

import (
	"time"

	"github.com/azzimoda/raspishika-go/pkg/tgbot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

const (
	ErrMsgTryLater            = "Произошла ошибка, попробуйте позже"
	ErrMsgFailedFetchSchedule = "Не удалось загрузить расписание, попробуйте позже"
)

func SendErrorMessage(api *tgbotapi.BotAPI, chatID int64, text string) error {
	err := tgbot.SendTempMessage(api, chatID, text, 10*time.Second)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send error message")
	}
	return err
}

func InlineButtonMarkupUpdate(kind string, groupName string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Обновить", "update_"+kind+"\n"+groupName)))
}

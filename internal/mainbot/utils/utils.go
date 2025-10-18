package utils

import (
	"github.com/azzimoda/raspishika-go/pkg/tgbot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	ErrMsgTryLater = "Произошла ошибка, попробуйте позже"
)

func SendErrorMessage(api *tgbotapi.BotAPI, chatID int64, text string) error {
	return tgbot.SendTempMessage(api, chatID, text, 10)
}

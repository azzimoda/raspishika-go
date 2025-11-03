package tgbot

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

type BotAPIProvider interface {
	API() *tgbotapi.BotAPI
}

type UpdateHandler interface {
	OnUpdate(tgbotapi.Update)
}

func StartPolling(bot interface {
	BotAPIProvider
	UpdateHandler
}) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.API().GetUpdatesChan(u)
	for update := range updates {
		go bot.OnUpdate(update)
	}
}

func SendTempMessage(api *tgbotapi.BotAPI, tgChatID int64, text string, dur time.Duration) error {
	newMsg := tgbotapi.NewMessage(tgChatID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	sentMsg, err := api.Send(newMsg)

	go func() {
		time.Sleep(dur)
		api.Send(tgbotapi.NewDeleteMessage(tgChatID, sentMsg.MessageID))
	}()

	return err
}

func SetMyCommands(api *tgbotapi.BotAPI, commandsData []map[string]string) {
	log.Trace().Any("commands", commandsData).Msg("Setting my commands")
	commands := make([]tgbotapi.BotCommand, len(commandsData))
	for i, cmd := range commandsData {
		for name, desc := range cmd {
			commands[i] = tgbotapi.BotCommand{Command: name, Description: desc}
		}
	}
	api.Send(tgbotapi.NewSetMyCommands(commands...))
}

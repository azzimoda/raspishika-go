package tgbot

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

type BotAPIProvider interface {
	API() *tgbotapi.BotAPI
}

type UpdateHandler interface {
	OnUpdate(tgbotapi.Update)
}

type CommandsProvider interface {
	MyCommands() []map[string]string
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

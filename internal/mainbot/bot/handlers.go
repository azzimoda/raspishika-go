package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (b *Bot) OnUpdate(update tgbotapi.Update) {
	if !b.ApplyMiddleware(update) {
		return
	}

	var err error
	switch {
	case update.Message != nil:
		err = b.onMessage(update.Message)
	case update.CallbackQuery != nil:
		err = b.onCallbackQuery(update.CallbackQuery)
	}
	if err != nil {
		log.Error().Err(err).Msg("Error while handling update")
	}
}

func (b *Bot) onMessage(msg *tgbotapi.Message) error {
	log.Debug().Str("text", msg.Text).Msg("Handling message")

	if msg.IsCommand() {
		return b.onCommand(msg)
	} else {
		return b.onText(msg)
	}
}

func (b *Bot) onCommand(msg *tgbotapi.Message) error {
	panic("unimplemented: onCommand")
}

func (b *Bot) onText(msg *tgbotapi.Message) error {
	panic("unimplemented: onText")
}

func (b *Bot) onCallbackQuery(msg *tgbotapi.CallbackQuery) error {
	panic("unimplemented: onCallbackQuery")
}

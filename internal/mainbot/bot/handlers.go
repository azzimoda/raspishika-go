package bot

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (b *Bot) OnUpdate(update tgbotapi.Update) {
	if !b.ApplyMiddleware(update, b.Repo) {
		return
	}

	var err error
	switch {
	case update.Message != nil:
		err = b.onMessage(update.Message)
	case update.CallbackQuery != nil:
		err = b.onCallbackQuery(update.CallbackQuery) // TODO
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
		return b.onText(msg) // TODO
	}
}

func (b *Bot) onCommand(msg *tgbotapi.Message) error {
	switch msg.Command() {
	case "start":
		return commands.OnStart(b.api, msg)
	case "help":
		return commands.OnHelp(b.api, msg)
	case "stop":
		return commands.OnStop(b.api, b.Repo, msg)

	case "settings":
		return commands.OnSettings(b.api, b.Repo, msg) // TODO
	case "group":
		return commands.OnGroup(b.api, b.Repo, b.Browser, b.Cache, msg) // TODO
	case "daily_time":
		return commands.OnDailyTime(b.api, b.Repo, msg) // TODO
	case "daily_off":
		return commands.OnDailyOff(b.api, b.Repo, msg) // TODO
	case "reminder_on":
		return commands.OnReminder(b.api, b.Repo, msg, true) // TODO
	case "reminder_off":
		return commands.OnReminder(b.api, b.Repo, msg, false)

	case "week":
		return commands.OnWeek(b.api, b.Repo, b.Browser, b.Cache, msg) // TODO
	case "tomorrow":
		return commands.OnTomorrow(b.api, b.Repo, b.Cache, msg) // TODO
	case "left":
		return commands.OnLeft(b.api, b.Repo, b.Cache, msg) // TODO

	case "quick":
		return commands.OnQuick(b.api, b.Repo, msg) // TODO
	case "teacher":
		return commands.OnTeacher(b.api, b.Repo, msg) // TODO
	default:
		return fmt.Errorf("Unexpected command: %s", msg.Command())
	}
}

func (b *Bot) onText(msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: Bot.onText")
}

func (b *Bot) onCallbackQuery(msg *tgbotapi.CallbackQuery) error {
	return fmt.Errorf("Unimplemented: Bot.onCallbackQuery")
}

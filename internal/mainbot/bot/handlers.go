package bot

import (
	"strings"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/callbacks"
	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"

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
	switch msg.Command() {
	case "start":
		err := commands.OnStart(b.api, msg)
		if err != nil {
			return err
		}

		if chat, err := b.Repo.GetChatByChatID(msg.Chat.ID); err == nil && chat.GroupName == nil {
			return commands.OnGroup(b.api, b.Repo, b.Browser, b.Cache, msg)
		} else {
			return err
		}
	case "help":
		return commands.OnHelp(b.api, msg)
	case "stop":
		return commands.OnStop(b.api, b.Repo, msg)

	case "settings":
		return commands.OnSettings(b.api, b.Repo, msg) // TODO
	case "group":
		return commands.OnGroup(b.api, b.Repo, b.Browser, b.Cache, msg)
	case "daily_time":
		return commands.OnDailyTime(b.api, b.Repo, msg) // TODO
	case "daily_off":
		return commands.OnDailyOff(b.api, b.Repo, msg) // TODO
	case "reminder_on":
		return commands.OnReminder(b.api, b.Repo, msg, true) // TODO
	case "reminder_off":
		return commands.OnReminder(b.api, b.Repo, msg, false)

	case "week":
		return commands.OnWeek(b.api, b.Repo, b.Browser, b.Cache, b.Config.Browser.ScreenshotDir,
			b.Config.ScheduleTemplate, msg)
	case "tomorrow":
		return commands.OnTomorrow(b.api, b.Repo, b.Cache, msg) // TODO
	case "left":
		return commands.OnLeft(b.api, b.Repo, b.Cache, msg) // TODO

	case "quick":
		return commands.OnQuick(b.api, b.Repo, msg) // TODO
	case "teacher":
		return commands.OnTeacher(b.api, b.Repo, msg) // TODO
	default:
		b.api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))
		log.Debug().Str("text", msg.Text).Msg("Unknown command")
		return nil
	}
}

func (b *Bot) onText(msg *tgbotapi.Message) error {
	chat, err := b.Repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(b.api, msg.Chat.ID, utils.ErrMsgTryLater)
		return err
	}

	switch chat.State {
	case database.ChatStateDefault:
		// TODO: Come up with some features here.
		return nil
	case database.ChatStateSelectingGroup:
		return commands.OnTextGroup(b.api, b.Repo, msg, chat)
	case database.ChatStateSelectingTime:
		return commands.OnTextTime(b.api, b.Repo, msg, chat)
	default:
		log.Warn().Str("state", string(chat.State)).Msg("Unknown state")
		if err := b.Repo.UpdateChatState(chat.ChatID, database.ChatStateDefault); err != nil {
			log.Error().Err(err).Msg("Error while updating chat state")
		}
		return nil
	}
}

func (b *Bot) onCallbackQuery(query *tgbotapi.CallbackQuery) error {
	log.Debug().Str("data", strings.ReplaceAll(query.Data, "\n", " // ")).Msg("Handling callback query")

	callbackCommand := ParseCallbackData(query.Data)
	log.Trace().Strs("args", callbackCommand.Args).Msgf("Command: %s", callbackCommand.Command)

	var err error
	switch callbackCommand.Command {
	case "delete":
		b.api.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	case "select_department":
		err = callbacks.OnSelectDepartment(b.api, b.Repo, b.Browser, b.Cache, query, callbackCommand.Args)

	case "update_group":
		err = callbacks.OnUpdateGroup(b.api, b.Repo, b.Browser, b.Cache, b.Config.Browser.ScreenshotDir,
			b.Config.ScheduleTemplate, query, callbackCommand.Args)

	default:
		b.api.Send(tgbotapi.NewCallback(query.ID, "?"))
		// err = nil
	}

	if err != nil {
		b.api.Send(tgbotapi.NewCallback(query.ID, utils.ErrMsgTryLater))
	}

	return err
}

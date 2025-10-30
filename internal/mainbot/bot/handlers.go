package bot

import (
	"fmt"
	"strings"
	"time"

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

	startTime := time.Now()
	updateLog := &database.UpdateLog{}

	var err error
	switch {
	case update.Message != nil:
		updateLog.ChatID = update.Message.Chat.ID
		updateLog.Kind = tgbotapi.UpdateTypeMessage
		updateLog.MessageID = update.Message.MessageID
		updateLog.Data = update.Message.Text

		err = b.onMessage(update.Message)
	case update.CallbackQuery != nil:
		updateLog.ChatID = update.CallbackQuery.Message.Chat.ID
		updateLog.Kind = tgbotapi.UpdateTypeCallbackQuery
		updateLog.MessageID = update.CallbackQuery.Message.MessageID
		updateLog.Data = update.CallbackQuery.Data

		err = b.onCallbackQuery(update.CallbackQuery)
	default:
		log.Warn().Msgf("Unknown update type: %T", update)
	}

	elapsed := time.Since(startTime)
	updateLog.HandlingTime = int(elapsed.Milliseconds())

	if err != nil {
		updateLog.Error = err.Error()
		log.Error().Err(err).Msg("Error while handling update")
		b.Report().Err(err).Chat(int64(updateLog.ChatID)).Send("Error while handling update") // TODO: .Debug("update", update)
	}
	log.Debug().Msgf("Update handled: %+v", updateLog)
	b.Repo.InsertUpdateLog(updateLog)
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
	b.api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	switch msg.Command() {
	case "start":
		return commands.OnStart(b.api, b.Repo, b.Browser, b.Cache, msg)
	case "help":
		return commands.OnHelp(b.api, msg)
	case "stop":
		return commands.OnStop(b.api, b.Repo, msg)

	case "settings":
		return commands.OnSettings(b.api, b.Repo, msg) // TODO
	case "group":
		return commands.OnGroup(b.api, b.Repo, b.Browser, b.Cache, msg)
	case "daily_time":
		return commands.OnDailyTime(b.api, b.Repo, msg)
	case "daily_off":
		return commands.OnDailyOff(b.api, b.Repo, msg)
	case "reminder_on":
		return commands.OnReminder(b.api, b.Repo, msg, true)
	case "reminder_off":
		return commands.OnReminder(b.api, b.Repo, msg, false)
	case "access":
		return commands.OnAccess(b.api, b.Repo, msg)

	case "week":
		return commands.OnWeek(b.api, b.Repo, b.Browser, b.Cache, b.Config.Browser.ScreenshotDir,
			b.Config.ScheduleTemplate, msg)
	case "tomorrow":
		return commands.OnTomorrow(b.api, b.Repo, b.Browser, b.Cache, msg)
	case "left":
		return commands.OnLeft(b.api, b.Repo, b.Browser, b.Cache, msg)

	case "quick":
		return commands.OnQuick(b.api, b.Repo, b.Cache, msg)
	case "teacher":
		return commands.OnTeacher(b.api, b.Repo, msg)
	default:
		log.Debug().Str("text", msg.Text).Msg("Unknown command")
		return nil
	}
}

func (b *Bot) onText(msg *tgbotapi.Message) error {
	chat, err := b.Repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(b.api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat state: %w", err)
	}

	switch chat.State {
	case database.ChatStateDefault:
		// TODO: Implement handling of reply buttons "Неделея", "Завтра", "Сегодня".
		return nil
	case database.ChatStateSelectingGroup:
		if strings.ToLower(msg.Text) == "отмена" {
			return b.onTextCancel(msg)
		}
		return commands.OnTextGroup(b.api, b.Repo, msg, chat)
	case database.ChatStateSelectingTime:
		return commands.OnTextTime(b.api, b.Repo, msg, chat)
	case database.ChatStateQuickSelectingGroup:
		if strings.ToLower(msg.Text) == "отмена" {
			return b.onTextCancel(msg)
		}
		return commands.OnTextQuickGroup(b.api, b.Repo, b.Browser, b.Cache,
			b.Config.Browser.ScreenshotDir, b.Config.ScheduleTemplate, msg)
	case database.ChatStateSelectingTeacher:
		return commands.OnTextTeacherName(b.api, b.Repo, b.Browser, msg)
	default:
		log.Warn().Str("state", string(chat.State)).Msg("Unknown state")
		if err := b.Repo.UpdateChatState(chat.ChatID, database.ChatStateDefault); err != nil {
			log.Error().Err(err).Msg("Failed to update chat state")
			return fmt.Errorf("failed to update chat state: %w", err)
		}
		return nil
	}
}

func (b *Bot) onTextCancel(msg *tgbotapi.Message) error {
	b.api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	if err := b.Repo.UpdateChatState(msg.Chat.ID, database.ChatStateDefault); err != nil {
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Действие отменено")
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
	sentMsg, err := b.api.Send(newMsg)
	go func() {
		time.Sleep(3 * time.Second)
		b.api.Send(tgbotapi.NewDeleteMessage(sentMsg.Chat.ID, sentMsg.MessageID))
	}()
	return err
}

func (b *Bot) onCallbackQuery(query *tgbotapi.CallbackQuery) error {
	log.Debug().Str("data", strings.ReplaceAll(query.Data, "\n", " // ")).Msg("Handling callback query")

	callbackCommand := ParseCallbackData(query.Data)
	log.Trace().Strs("args", callbackCommand.Args).Msgf("Command: %s", callbackCommand.Command)

	var err error
	switch callbackCommand.Command {
	case "delete":
		b.api.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
		if err := b.Repo.UpdateChatState(query.Message.Chat.ID, database.ChatStateDefault); err != nil {
			return fmt.Errorf("failed to update chat state: %w", err)
		}

	case "select_department":
		err = callbacks.OnSelectDepartment(b.api, b.Repo, b.Browser, b.Cache, query, callbackCommand.Args)
	case "quick_select_department":
		err = callbacks.OnQuickSelectDepartment(b.api, b.Repo, b.Browser, b.Cache, query, callbackCommand.Args)
	case "select_teacher":
		err = callbacks.OnSelectTeacher(b.api, b.Repo, b.Browser, b.Cache, b.Config.Browser.ScreenshotDir,
			b.Config.ScheduleTemplate, query, callbackCommand.Args)

	case "set_access":
		err = callbacks.OnSetAccess(b.api, b.Repo, query, callbackCommand.Args)

	case "update_group":
		err = callbacks.OnUpdateGroup(b.api, b.Repo, b.Browser, b.Cache, b.Config.Browser.ScreenshotDir,
			b.Config.ScheduleTemplate, query, callbackCommand.Args)
	case "update_teacher":
		err = callbacks.OnUpdateTeacher(b.api, b.Repo, b.Browser, b.Cache, b.Config.Browser.ScreenshotDir,
			b.Config.ScheduleTemplate, query, callbackCommand.Args)
	case "update_tomorrow":
		err = callbacks.OnUpdateTomorrow(b.api, b.Repo, b.Cache, query, callbackCommand.Args)
	case "update_left":
		err = callbacks.OnUpdateLeft(b.api, b.Repo, b.Cache, query, callbackCommand.Args)

	default:
		b.api.Send(tgbotapi.NewCallback(query.ID, "?"))
		// err = nil
	}

	if err != nil {
		b.api.Send(tgbotapi.NewCallback(query.ID, utils.ErrMsgTryLater))
	}
	return err
}

package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (b *Bot) OnUpdate(update tgbotapi.Update) {
	if !b.ApplyMiddleware(update, b.repo) {
		return
	}

	startTime := time.Now()
	updateLog := &database.UpdateLog{}

	var chat *database.Chat
	var err error
	switch {
	case update.Message != nil:
		chat, err = b.repo.GetChatByTgChatID(update.Message.Chat.ID)
		if err != nil {
			log.Error().Err(err).Int64("tgChatID", update.Message.Chat.ID).
				Msg("failed to get chat by chat ID")
		}

		updateLog.ChatID = chat.ID
		updateLog.Kind = tgbotapi.UpdateTypeMessage
		updateLog.MessageID = update.Message.MessageID
		updateLog.Data = update.Message.Text

		err = b.onMessage(update.Message)
	case update.CallbackQuery != nil:
		chat, err = b.repo.GetChatByTgChatID(update.CallbackQuery.Message.Chat.ID)
		if err != nil {
			log.Error().Err(err).Int64("tgChatID", update.CallbackQuery.Message.Chat.ID).
				Msg("failed to get chat by chat ID")
		}

		updateLog.ChatID = chat.ID
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
		b.Report().Err(err).Chat(int64(chat.TgChatID)).Send("Error while handling update") // TODO: .Debug("update", update)
	}
	log.Debug().Dur("elapsed", elapsed).Str("kind", updateLog.Kind).Msg("Update handled")
	b.repo.InsertUpdateLog(updateLog)
}

func (b *Bot) onMessage(msg *tgbotapi.Message) error {
	log.Debug().Int64("tgChatID", msg.Chat.ID).Str("username", msg.Chat.UserName).Str("text", msg.Text).Msg("Handling message")

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
		return b.CommandHandler.OnStart(msg)
	case "help":
		return b.CommandHandler.OnHelp(msg)
	case "stop":
		return b.CommandHandler.OnStop(msg)

	case "settings":
		return b.CommandHandler.OnSettings(msg)
	case "group":
		return b.CommandHandler.OnGroup(msg)
	case "daily_time":
		return b.CommandHandler.OnDailyTime(msg)
	case "daily_off":
		return b.CommandHandler.OnDailyOff(msg)
	case "reminder_on":
		return b.CommandHandler.OnReminder(msg, true)
	case "reminder_off":
		return b.CommandHandler.OnReminder(msg, false)
	case "access":
		return b.CommandHandler.OnAccess(msg)

	case "week":
		return b.CommandHandler.OnWeek(msg)
	case "tomorrow":
		return b.CommandHandler.OnTomorrow(msg)
	case "left":
		return b.CommandHandler.OnLeft(msg)

	case "quick":
		return b.CommandHandler.OnQuick(msg)
	case "teacher":
		return b.CommandHandler.OnTeacher(msg)
	default:
		log.Debug().Str("text", msg.Text).Msg("Unknown command")
		return nil
	}
}

func (b *Bot) onText(msg *tgbotapi.Message) error {
	chat, err := b.repo.GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(b.api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat state: %w", err)
	}

	switch chat.State {
	case database.ChatStateDefault:
		lowerText := strings.ToLower(msg.Text)

		if chat.IsPrivate() {
			switch lowerText {
			case "неделя":
				return b.CommandHandler.OnWeek(msg)
			case "завтра":
				return b.CommandHandler.OnTomorrow(msg)
			case "сегодня":
				return b.CommandHandler.OnLeft(msg)
			case "другая группа":
				return b.CommandHandler.OnQuick(msg)
			case "преподаватель":
				return b.CommandHandler.OnTeacher(msg)
			}
		}

		return nil
	case database.ChatStateSelectingGroup:
		if strings.ToLower(msg.Text) == "отмена" {
			return b.onTextCancel(msg)
		}
		return b.CommandHandler.OnTextGroup(msg, chat)
	case database.ChatStateSelectingTime:
		return b.CommandHandler.OnTextTime(msg, chat)
	case database.ChatStateQuickSelectingGroup:
		if strings.ToLower(msg.Text) == "отмена" {
			return b.onTextCancel(msg)
		}
		return b.CommandHandler.OnTextQuickGroup(msg)
	case database.ChatStateSelectingTeacher:
		return b.CommandHandler.OnTextTeacherName(msg)
	default:
		log.Warn().Str("state", string(chat.State)).Msg("Unknown state")
		if err := b.repo.UpdateChatState(chat.TgChatID, database.ChatStateDefault); err != nil {
			log.Error().Err(err).Msg("Failed to update chat state")
			return fmt.Errorf("failed to update chat state: %w", err)
		}
		return nil
	}
}

func (b *Bot) onTextCancel(msg *tgbotapi.Message) error {
	b.api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	if err := b.repo.UpdateChatState(msg.Chat.ID, database.ChatStateDefault); err != nil {
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Действие отменено")
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = utils.MainMenuReplyMarkup(msg.Chat.IsPrivate())
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
		if err := b.repo.UpdateChatState(query.Message.Chat.ID, database.ChatStateDefault); err != nil {
			return fmt.Errorf("failed to update chat state: %w", err)
		}
	// TODO: Add "delete_config"

	case "config_group":
		err = b.CallbackHandler.OnConfigGroup(b.CommandHandler, query, callbackCommand.Args)
	case "config_daily_time":
		err = b.CallbackHandler.OnConfigDailyTime(b.CommandHandler, query, callbackCommand.Args)
	case "daily_off":
		err = b.CallbackHandler.OnDailyOff(query, callbackCommand.Args)
	case "config_reminder":
		err = b.CallbackHandler.OnConfigReminder(b.CommandHandler, query, callbackCommand.Args)
	case "config_access":
		err = b.CallbackHandler.OnConfigAccess(b.CommandHandler, query, callbackCommand.Args)

	case "select_department":
		err = b.CallbackHandler.OnSelectDepartment(query, callbackCommand.Args)
	case "quick_select_department":
		err = b.CallbackHandler.OnQuickSelectDepartment(query, callbackCommand.Args)
	case "select_teacher":
		err = b.CallbackHandler.OnSelectTeacher(b.CommandHandler, query, callbackCommand.Args)

	case "set_access":
		err = b.CallbackHandler.OnSetAccess(query, callbackCommand.Args)

	case "update_group":
		err = b.CallbackHandler.OnUpdateGroup(query, callbackCommand.Args)
	case "update_teacher":
		err = b.CallbackHandler.OnUpdateTeacher(query, callbackCommand.Args)
	case "update_tomorrow":
		err = b.CallbackHandler.OnUpdateTomorrow(query, callbackCommand.Args)
	case "update_left":
		err = b.CallbackHandler.OnUpdateLeft(query, callbackCommand.Args)

	default:
		b.api.Send(tgbotapi.NewCallback(query.ID, "?"))
		// err = nil
	}

	if err != nil {
		b.api.Send(tgbotapi.NewCallback(query.ID, utils.ErrMsgTryLater))
	}
	return err
}

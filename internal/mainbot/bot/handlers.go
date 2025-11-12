package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/database"
	botutils "github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/pkg/utils"

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
	var handled bool
	var err error
	switch {
	case update.Message != nil:
		chat, err = b.repo.GetChatByTgChatID(update.Message.Chat.ID)
		if err != nil {
			log.Error().Err(err).Int64("tgChatID", update.Message.Chat.ID).
				Msg("failed to get chat by chat ID")
			return
		} else {
			updateLog.ChatID = chat.ID
			updateLog.Kind = tgbotapi.UpdateTypeMessage
			updateLog.MessageID = update.Message.MessageID
			updateLog.Data = update.Message.Text
		}

		handled, err = b.onMessage(update.Message)
	case update.CallbackQuery != nil:
		chat, err = b.repo.GetChatByTgChatID(update.CallbackQuery.Message.Chat.ID)
		if err != nil || chat == nil {
			log.Error().Err(err).Int64("tgChatID", update.CallbackQuery.Message.Chat.ID).
				Msg("failed to get chat by chat ID")
		} else {
			updateLog.ChatID = chat.ID
			updateLog.Kind = tgbotapi.UpdateTypeCallbackQuery
			updateLog.MessageID = update.CallbackQuery.Message.MessageID
			updateLog.Data = update.CallbackQuery.Data
		}

		handled, err = b.onCallbackQuery(update.CallbackQuery)
	default:
		log.Warn().Msgf("Unknown update type: %T", update)
	}

	elapsed := time.Since(startTime)
	updateLog.HandlingTime = int(elapsed.Milliseconds())

	if chat == nil {
		chat = &database.Chat{}
	}

	if handled {
		log.Info().
			Int64("tgChatID", chat.TgChatID).
			Str("username", utils.DerefOrTypeDefault(chat.UserName)).
			Str("kind", updateLog.Kind).
			Str("data", shortenText(updateLog.Data, 64)).
			Dur("elapsed", elapsed).
			Msg("Update handled")
	} else {
		log.Trace().
			Int64("tgChatID", chat.TgChatID).
			Str("username", utils.DerefOrTypeDefault(chat.UserName)).
			Str("kind", updateLog.Kind).
			Str("data", shortenText(updateLog.Data, 64)).
			Dur("elapsed", elapsed).
			Msg("Update not handled")
	}

	if err != nil {
		errStr := err.Error()
		updateLog.Error = &errStr
		log.Error().Err(err).Msg("Error while handling update")
		b.Report().Err(err).Chat(chat).Send("Error while handling update") // TODO: .Debug("update", update)
	}
	b.repo.InsertUpdateLog(updateLog)
}

func (b *Bot) onMessage(msg *tgbotapi.Message) (bool, error) {
	log.Debug().
		Int64("tgChatID", msg.Chat.ID).
		Str("username", msg.Chat.UserName).
		Str("text", msg.Text).
		Msg("Handling message")

	if msg.IsCommand() {
		return b.onCommand(msg)
	} else {
		return b.onText(msg)
	}
}

func (b *Bot) onCommand(msg *tgbotapi.Message) (handled bool, err error) {
	b.api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	handled = true
	switch msg.Command() {
	case "start":
		err = b.CommandHandler.OnStart(msg)
	case "help":
		err = b.CommandHandler.OnHelp(msg)
	case "stop":
		err = b.CommandHandler.OnStop(msg)

	case "settings":
		err = b.CommandHandler.OnSettings(msg)
	case "group":
		err = b.CommandHandler.OnGroup(msg)
	case "daily_time":
		err = b.CommandHandler.OnDailyTime(msg)
	case "daily_off":
		err = b.CommandHandler.OnDailyOff(msg)
	case "reminder_on":
		err = b.CommandHandler.OnReminder(msg, true)
	case "reminder_off":
		err = b.CommandHandler.OnReminder(msg, false)
	case "access":
		err = b.CommandHandler.OnAccess(msg)

	case "week":
		err = b.CommandHandler.OnWeek(msg)
	case "tomorrow":
		err = b.CommandHandler.OnTomorrow(msg)
	case "left":
		err = b.CommandHandler.OnLeft(msg)

	case "quick":
		err = b.CommandHandler.OnQuick(msg)
	case "teacher":
		err = b.CommandHandler.OnTeacher(msg)
	default:
		log.Debug().Str("text", msg.Text).Msg("Unknown command")
		handled = false
	}
	return
}

func (b *Bot) onText(msg *tgbotapi.Message) (handled bool, err error) {
	chat, err := b.repo.GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(b.api, msg.Chat.ID, botutils.ErrMsgTryLater)
		return false, fmt.Errorf("failed to get chat state: %w", err)
	}

	handled = true
	switch chat.State {
	case database.ChatStateDefault:
		if chat.IsPrivate() {
			switch strings.ToLower(msg.Text) {
			case "неделя":
				err = b.CommandHandler.OnWeek(msg)
			case "завтра":
				err = b.CommandHandler.OnTomorrow(msg)
			case "сегодня":
				err = b.CommandHandler.OnLeft(msg)
			case "другая группа":
				err = b.CommandHandler.OnQuick(msg)
			case "преподаватель":
				err = b.CommandHandler.OnTeacher(msg)
			default:
				handled = false
			}
		}
	case database.ChatStateSelectingGroup:
		if strings.ToLower(msg.Text) == "отмена" {
			err = b.onTextCancel(msg)
		} else {
			err = b.CommandHandler.OnTextGroup(msg, chat)
		}
	case database.ChatStateSelectingTime:
		err = b.CommandHandler.OnTextTime(msg, chat)
	case database.ChatStateQuickSelectingGroup:
		if strings.ToLower(msg.Text) == "отмена" {
			err = b.onTextCancel(msg)
		} else {
			err = b.CommandHandler.OnTextQuickGroup(msg)
		}
	case database.ChatStateSelectingTeacher:
		err = b.CommandHandler.OnTextTeacherName(msg)

	default:
		log.Warn().Str("state", string(chat.State)).Msg("Unknown state")
		if err := b.repo.UpdateChatState(chat.TgChatID, database.ChatStateDefault); err != nil {
			log.Error().Err(err).Msg("Failed to update chat state")
			err = fmt.Errorf("failed to update chat state: %w", err)
		}
	}
	return
}

func (b *Bot) onTextCancel(msg *tgbotapi.Message) error {
	b.api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	if err := b.repo.UpdateChatState(msg.Chat.ID, database.ChatStateDefault); err != nil {
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Действие отменено")
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = botutils.MainMenuReplyMarkup(msg.Chat.IsPrivate())
	sentMsg, err := b.api.Send(newMsg)
	go func() {
		time.Sleep(3 * time.Second)
		b.api.Send(tgbotapi.NewDeleteMessage(sentMsg.Chat.ID, sentMsg.MessageID))
	}()
	return err
}

func (b *Bot) onCallbackQuery(query *tgbotapi.CallbackQuery) (bool, error) {
	log.Debug().
		Int64("tgChatID", query.Message.Chat.ID).
		Str("username", query.Message.Chat.UserName).
		Str("data", strings.ReplaceAll(query.Data, "\n", " // ")).
		Msg("Handling callback query")

	callbackCommand := ParseCallbackData(query.Data)
	log.Trace().Strs("args", callbackCommand.Args).Msgf("Command: %s", callbackCommand.Command)

	var handled bool = true
	var err error
	switch callbackCommand.Command {
	case "delete":
		b.api.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
		if err := b.repo.UpdateChatState(query.Message.Chat.ID, database.ChatStateDefault); err != nil {
			return false, fmt.Errorf("failed to update chat state: %w", err)
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
		handled = false
	}

	if err != nil {
		b.api.Send(tgbotapi.NewCallback(query.ID, botutils.ErrMsgTryLater))
	}
	return handled, err
}

func (b *Bot) handleTelegramAPIError(tgErr *tgbotapi.Error, chat *database.Chat) bool {
	if strings.Contains(strings.ToLower(tgErr.Message), "forbidden") {
		log.Warn().Int64("tgChatID", chat.TgChatID).Msg("Forbidden, deleting chat")
		b.repo.DeleteChat(chat.ID)
		return true
	}
	return false
}

func shortenText(text string, maxLength int) string {
	if len(text) > maxLength {
		return text[:maxLength-2] + "…"
	}
	return text
}

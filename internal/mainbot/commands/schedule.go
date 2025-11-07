package commands

import (
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/azzimoda/raspishika-go/internal/database"
	botutils "github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (ch *CommandHandler) OnWeek(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Msg("Group not set, offering to set group")
		return ch.OnGroup(msg)
	}

	group, shouldReturn, err := ch.tryGetGroup(chat, msg)
	if shouldReturn {
		return err
	}

	scheduleCfg := scraper.GroupScheduleConfig(group)
	return ch.SendWeekSchedule(chat, scheduleCfg)
}

func (ch *CommandHandler) SendWeekSchedule(chat *database.Chat, scheduleCfg scraper.ScheduleConfig) error {
	var schedule *scraper.RawSchedule
	var err error

	schedule, err = ch.Bot.ScheduleManager().Get(ch.Bot.Repo(), ch.Bot.Browser(), ch.Bot.Cache(), scheduleCfg)
	if err != nil {
		// TODO: Try to send old photo on error.
		botutils.SendErrorMessage(ch.Bot.API(), chat.TgChatID, botutils.ErrMsgFailedFetchSchedule)
		return fmt.Errorf("failed to fetch schedule: %w", err)
	}

	html := schedule.HTML(ch.Bot.Cache(), ch.Bot.Config().ScheduleTemplate)
	imagePath := path.Join(ch.Bot.Config().Browser.ScreenshotDir, botutils.ScheduleScreenshotFileName(scheduleCfg))
	if err := ch.Bot.Browser().TakeScreenshotHTML(html, imagePath); err != nil {
		// TODO: Try to send old photo on error.
		botutils.SendErrorMessage(ch.Bot.API(), chat.TgChatID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to take screenshot of schedule; %w", err)
	}

	ch.Bot.API().Send(tgbotapi.NewChatAction(chat.TgChatID, tgbotapi.ChatUploadPhoto))

	newMsg := tgbotapi.NewMessage(chat.TgChatID, scheduleCfg.FormatMarkdown()+":")
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = botutils.MainMenuReplyMarkup(chat.IsPrivate())
	_, err2 := ch.Bot.API().Send(newMsg)

	newPhotoMsg := tgbotapi.NewPhoto(chat.TgChatID, tgbotapi.FilePath(imagePath))
	newPhotoMsg.ReplyMarkup = weekScheduleInlineButtonMarkup(scheduleCfg)
	_, err1 := ch.Bot.API().Send(newPhotoMsg)

	return errors.Join(err1, err2)
}

func weekScheduleInlineButtonMarkup(config scraper.ScheduleConfig) tgbotapi.InlineKeyboardMarkup {
	if config.Group != nil {
		return botutils.InlineButtonMarkupUpdate("group", config.Group.GroupName)
	} else if config.Teacher != nil {
		return botutils.InlineButtonMarkupUpdate("teacher", config.Teacher.TeacherID)
	} else {
		panic("unreachable")
	}
}

func (ch *CommandHandler) OnTomorrow(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Msg("Group not set, offering to set group")
		return ch.OnGroup(msg)
	}
	group, shouldReturn, err := ch.tryGetGroup(chat, msg)
	if shouldReturn {
		return err
	}

	rawSchedule, err := ch.Bot.ScheduleManager().Get(
		ch.Bot.Repo(),
		ch.Bot.Browser(),
		ch.Bot.Cache(),
		scraper.GroupScheduleConfig(group),
	)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgFailedFetchSchedule)
		return fmt.Errorf("failed to fetch schedule of group %s: %w", group.GroupName, err)
	}
	schedule := rawSchedule.Transform()

	var tomorrow scraper.ScheduleDay
	if time.Now().Weekday() == time.Sunday {
		tomorrow = schedule.Days[0]
	} else {
		tomorrow = schedule.Days[1]
	}

	text := tomorrow.String()
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = botutils.InlineButtonMarkupUpdate("tomorrow", *chat.GroupName)

	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func (ch *CommandHandler) OnLeft(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
	}

	if chat.GroupName == nil {
		log.Warn().Msg("Group not set, offering to set group")
		return ch.OnGroup(msg)
	}

	if time.Now().Weekday() == time.Sunday {
		newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Сегодня воскресенье, отдыхайте!")
		newMsg.ReplyMarkup = botutils.InlineButtonMarkupUpdate("left", *chat.GroupName)
		_, err := ch.Bot.API().Send(newMsg)
		return err
	}

	group, shouldReturn, err := ch.tryGetGroup(chat, msg)
	if shouldReturn {
		return err
	}

	rawSchedule, err := ch.Bot.ScheduleManager().Get(
		ch.Bot.Repo(),
		ch.Bot.Browser(),
		ch.Bot.Cache(),
		scraper.GroupScheduleConfig(group),
	)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgFailedFetchSchedule)
		return fmt.Errorf("failed to fetch schedule of group %s: %w", group.GroupName, err)
	}

	schedule := rawSchedule.Transform()
	left := schedule.Days[0].Left()
	text := ""
	if left.IsEmpty() {
		text = "Сегодня больше нет пар"
	} else {
		text = left.String()
	}
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = botutils.InlineButtonMarkupUpdate("left", *chat.GroupName)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

// TODO: Move this function somewhere else and use everywhere,
func (ch *CommandHandler) tryGetGroup(chat *database.Chat, msg *tgbotapi.Message) (*database.Group, bool, error) {
	group, err := botutils.FetchGroupByNameWithVadiation(ch.Bot.Repo(), ch.Bot.Browser(), ch.Bot.Cache(), *chat.GroupName)
	if err == nil {
		return group, false, nil
	}

	switch {
	case errors.Is(err, botutils.ErrWrongGroupNameFormat):
		// Should be impossible, since group name is validated before setting it to chat.
		log.Warn().Int64("tgChatID", chat.TgChatID).Str("groupName", *chat.GroupName).Msg("Wrong group name format, offer to set group again")

		chat.DepartmentName = nil
		chat.GroupName = nil
		if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
			botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
			return nil, true, fmt.Errorf("failed to update chat: %w", err)
		}

		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgSelectGroupAgain)
		return nil, true, ch.OnGroup(msg)
	case errors.Is(err, botutils.ErrGroupNotFound):
		// Group not found, offer to set group again.
		chat.DepartmentName = nil
		chat.GroupName = nil
		if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
			botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
			return nil, true, fmt.Errorf("failed to update chat: %w", err)
		}

		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgSelectGroupAgain)
		return nil, true, ch.OnGroup(msg)
	default:
		// Any other error, return error.
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return nil, true, fmt.Errorf("failed to fetch group: %w", err)
	}
}

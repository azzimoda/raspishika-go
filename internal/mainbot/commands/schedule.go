package commands

import (
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (ch *CommandHandler) OnWeek(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Msg("Group not set, offering to set group")
		return ch.OnGroup(msg)
	}

	group, err := ch.Bot.Repo().GetGroupByName(*chat.GroupName)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", *chat.GroupName, err)
	}
	scheduleCfg := scraper.GroupScheduleConfig(group)
	return ch.SendWeekSchedule(msg.Chat.ID, scheduleCfg)
}

func (ch *CommandHandler) SendWeekSchedule(chatID int64, scheduleCfg scraper.ScheduleConfig) error {
	var schedule *scraper.RawSchedule
	var err error

	repo := ch.Bot.Repo()
	cache := ch.Bot.Cache()

	switch {
	case scheduleCfg.Group != nil:
		cacheConfig := cache.Config
		schedule, err = scraper.FetchSchedule(repo, cacheConfig.Dir, scheduleCfg)
	case scheduleCfg.Teacher != nil:
		schedule, err = scraper.FetchScheduleWithBrowser(repo, ch.Bot.Browser(), scheduleCfg)
	}
	if err != nil {
		// TODO: Try to send old photo on error.
		utils.SendErrorMessage(ch.Bot.API(), chatID, utils.ErrMsgFailedFetchSchedule)
		return fmt.Errorf("failed to fetch schedule: %w", err)
	}

	html := schedule.HTML(cache, ch.Bot.Config().ScheduleTemplate)
	imagePath := path.Join(ch.Bot.Config().Browser.ScreenshotDir, utils.ScheduleScreenshotFileName(scheduleCfg))
	if err := ch.Bot.Browser().TakeScreenshotHTML(html, imagePath); err != nil {
		// TODO: Try to send old photo on error.
		utils.SendErrorMessage(ch.Bot.API(), chatID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to take screenshot of schedule; %w", err)
	}

	ch.Bot.API().Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatUploadPhoto))

	newPhotoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(imagePath))
	newPhotoMsg.ReplyMarkup = weekScheduleInlineButtonMarkup(scheduleCfg)
	_, err1 := ch.Bot.API().Send(newPhotoMsg)

	newMsg := tgbotapi.NewMessage(chatID, scheduleCfg.FormatMarkdown())
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
	_, err2 := ch.Bot.API().Send(newMsg)

	return errors.Join(err1, err2)
}

func weekScheduleInlineButtonMarkup(config scraper.ScheduleConfig) tgbotapi.InlineKeyboardMarkup {
	if config.Group != nil {
		return utils.InlineButtonMarkupUpdate("group", config.Group.GroupName)
	} else if config.Teacher != nil {
		return utils.InlineButtonMarkupUpdate("teacher", config.Teacher.TeacherID)
	} else {
		panic("unreachable")
	}
}

func (ch *CommandHandler) OnTomorrow(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Msg("Group not set, offering to set group")
		return ch.OnGroup(msg)
	}
	group, err := ch.Bot.Repo().GetGroupByName(*chat.GroupName)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return err
	}

	rawSchedule, err := scraper.FetchSchedule(ch.Bot.Repo(), ch.Bot.Cache().Config.Dir, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgFailedFetchSchedule)
		return err
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
	newMsg.ReplyMarkup = utils.InlineButtonMarkupUpdate("tomorrow", *chat.GroupName)

	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func (ch *CommandHandler) OnLeft(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
	}

	if chat.GroupName == nil {
		log.Warn().Msg("Group not set, offering to set group")
		return ch.OnGroup(msg)
	}

	if time.Now().Weekday() == time.Sunday {
		newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Сегодня воскресенье, отдыхайте!")
		newMsg.ReplyMarkup = utils.InlineButtonMarkupUpdate("left", *chat.GroupName)
		_, err := ch.Bot.API().Send(newMsg)
		return err
	}

	group, err := ch.Bot.Repo().GetGroupByName(*chat.GroupName)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgFailedFetchSchedule)
		return err
	}

	rawSchedule, err := scraper.FetchSchedule(ch.Bot.Repo(), ch.Bot.Cache().Config.Dir, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgFailedFetchSchedule)
		return err
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
	newMsg.ReplyMarkup = utils.InlineButtonMarkupUpdate("left", *chat.GroupName)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

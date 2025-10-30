package commands

import (
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func OnWeek(
	api *tgbotapi.BotAPI, repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
	screenshotDir string, templateFile string, msg *tgbotapi.Message,
) error {
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Msg("Group not set, offering to set group")
		return OnGroup(api, repo, browser, cache, msg)
	}

	group, err := repo.GetGroupByName(*chat.GroupName)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", *chat.GroupName, err)
	}
	scheduleCfg := scraper.GroupScheduleConfig(group)
	return SendWeekSchedule(api, repo, browser, cache, screenshotDir, templateFile, msg.Chat.ID, scheduleCfg)
}

func SendWeekSchedule(
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	screenshotDir, templateFile string,
	chatID int64,
	scheduleCfg scraper.ScheduleConfig, // TODO: Change the function to take ScheduleConfig instead of a group name.
) error {
	var schedule *scraper.RawSchedule
	var err error
	switch {
	case scheduleCfg.Group != nil:
		schedule, err = scraper.FetchSchedule(repo, scheduleCfg)
	case scheduleCfg.Teacher != nil:
		schedule, err = scraper.FetchScheduleWithBrowser(repo, browser, scheduleCfg)
	}
	if err != nil {
		// TODO: Try to send old photo on error.
		utils.SendErrorMessage(api, chatID, utils.ErrMsgFailedFetchSchedule)
		return fmt.Errorf("failed to fetch schedule: %w", err)
	}

	html := schedule.HTML(cache, templateFile)
	imagePath := path.Join(screenshotDir, utils.ScheduleScreenshotFileName(scheduleCfg))
	if err := browser.TakeScreenshotHTML(html, imagePath); err != nil {
		// TODO: Try to send old photo on error.
		utils.SendErrorMessage(api, chatID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to take screenshot of schedule; %w", err)
	}

	api.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatUploadPhoto))

	newPhotoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(imagePath))
	newPhotoMsg.ReplyMarkup = weekScheduleInlineButtonMarkup(scheduleCfg)
	_, err1 := api.Send(newPhotoMsg)

	newMsg := tgbotapi.NewMessage(chatID, scheduleCfg.FormatMarkdown())
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
	_, err2 := api.Send(newMsg)

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

func OnTomorrow(
	api *tgbotapi.BotAPI, repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
	msg *tgbotapi.Message,
) error {
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Msg("Group not set, offering to set group")
		return OnGroup(api, repo, browser, cache, msg)
	}
	group, err := repo.GetGroupByName(*chat.GroupName)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return err
	}

	rawSchedule, err := scraper.FetchSchedule(repo, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgFailedFetchSchedule)
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

	_, err = api.Send(newMsg)
	return err
}

func OnLeft(
	api *tgbotapi.BotAPI, repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
	msg *tgbotapi.Message,
) error {
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
	}

	if chat.GroupName == nil {
		log.Warn().Msg("Group not set, offering to set group")
		return OnGroup(api, repo, browser, cache, msg)
	}

	if time.Now().Weekday() == time.Sunday {
		newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Сегодня воскресенье, отдыхайте!")
		newMsg.ReplyMarkup = utils.InlineButtonMarkupUpdate("left", *chat.GroupName)
		_, err := api.Send(newMsg)
		return err
	}

	group, err := repo.GetGroupByName(*chat.GroupName)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgFailedFetchSchedule)
		return err
	}

	rawSchedule, err := scraper.FetchSchedule(repo, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgFailedFetchSchedule)
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
	_, err = api.Send(newMsg)
	return err
}

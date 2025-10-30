package callbacks

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/rs/zerolog/log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (ch *CallbackHandler) OnUpdateGroup(query *tgbotapi.CallbackQuery, args []string) error {
	groupName := args[0]
	group, err := ch.Bot.Repo().GetGroupByName(groupName)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", groupName, err)
	}

	schedule, err := scraper.FetchSchedule(ch.Bot.Repo(), ch.Bot.Cache().Config.Dir, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgFailedFetchSchedule)
		return err
	}

	html := schedule.HTML(ch.Bot.Cache(), ch.Bot.Config().ScheduleTemplate)

	imagePath := path.Join(ch.Bot.Config().Browser.ScreenshotDir, fmt.Sprintf("schedule_%s.png", group.GroupName))
	if err := ch.Bot.Browser().TakeScreenshotHTML(html, imagePath); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return err
	}

	markup := utils.InlineButtonMarkupUpdate("group", groupName)
	media := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(imagePath))
	editConfig := tgbotapi.EditMessageMediaConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			ReplyMarkup: &markup,
		},
		Media: media,
	}
	_, err = ch.Bot.API().Send(editConfig)
	return err
}

func (ch *CallbackHandler) OnUpdateTeacher(query *tgbotapi.CallbackQuery, args []string) error {
	teacherID := args[0]
	teacher, err := ch.Bot.Repo().GetTeacherByTeacherID(teacherID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get teacher by teacher id (%s): %w", teacherID, err)
	}

	scheduleConfig := scraper.TeacherScheduleConfig(teacher)
	schedule, err := scraper.FetchScheduleWithBrowser(ch.Bot.Repo(), ch.Bot.Browser(), scheduleConfig)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch schedule of teacher (%s): %w", teacherID, err)
	}

	html := schedule.HTML(ch.Bot.Cache(), ch.Bot.Config().ScheduleTemplate)
	imagePath := path.Join(ch.Bot.Config().Browser.ScreenshotDir, utils.ScheduleScreenshotFileName(scheduleConfig))
	if err := ch.Bot.Browser().TakeScreenshotHTML(html, imagePath); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to take screenshot of schedule: %w", err)
	}

	markup := utils.InlineButtonMarkupUpdate("teacher", teacherID)
	media := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(imagePath))
	editConfig := tgbotapi.EditMessageMediaConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:      query.Message.Chat.ID,
			MessageID:   query.Message.MessageID,
			ReplyMarkup: &markup,
		},
		Media: media,
	}
	_, err = ch.Bot.API().Send(editConfig)
	return err
}

func (ch *CallbackHandler) OnUpdateTomorrow(query *tgbotapi.CallbackQuery, args []string) error {
	groupName := args[0]
	group, err := ch.Bot.Repo().GetGroupByName(groupName)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", groupName, err)
	}

	rawSchedule, err := scraper.FetchSchedule(ch.Bot.Repo(), ch.Bot.Cache().Config.Dir, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgFailedFetchSchedule)
		return err
	}
	schedule := rawSchedule.Transform()

	var tomorrow scraper.ScheduleDay
	if time.Now().Weekday() == time.Sunday {
		tomorrow = schedule.Days[0]
	} else {
		tomorrow = schedule.Days[1]
	}

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		query.Message.Chat.ID,
		query.Message.MessageID,
		tomorrow.String(),
		utils.InlineButtonMarkupUpdate("tomorrow", groupName),
	)
	editMsg.ParseMode = "MarkdownV2"
	_, err = ch.Bot.API().Send(editMsg)

	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		log.Warn().Int64("chatID", query.Message.Chat.ID).Msg("Message is not modified")
		ch.Bot.API().Send(tgbotapi.NewCallback(query.ID, "Ничего не изменилось"))
		return nil
	}
	return err
}

func (ch *CallbackHandler) OnUpdateLeft(query *tgbotapi.CallbackQuery, args []string) error {
	groupName := args[0]
	group, err := ch.Bot.Repo().GetGroupByName(groupName)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", groupName, err)
	}

	rawSchedule, err := scraper.FetchSchedule(ch.Bot.Repo(), ch.Bot.Cache().Config.Dir, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgFailedFetchSchedule)
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
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		query.Message.Chat.ID,
		query.Message.MessageID,
		text,
		utils.InlineButtonMarkupUpdate("left", groupName),
	)
	editMsg.ParseMode = "MarkdownV2"
	_, err = ch.Bot.API().Send(editMsg)

	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		log.Warn().Int64("chatID", query.Message.Chat.ID).Msg("Message is not modified")
		ch.Bot.API().Send(tgbotapi.NewCallback(query.ID, "Ничего не изменилось"))
		return nil
	}
	return err
}

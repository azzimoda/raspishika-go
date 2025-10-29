package callbacks

import (
	"fmt"
	"path"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func OnUpdateGroup(
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	screenshotDir, templateFile string,
	query *tgbotapi.CallbackQuery,
	args []string,
) error {
	groupName := args[0]
	group, err := repo.GetGroupByName(groupName)
	if err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", groupName, err)
	}

	schedule, err := scraper.FetchSchedule(repo, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgFailedFetchSchedule)
		return err
	}

	html := schedule.HTML(cache, templateFile)

	imagePath := path.Join(screenshotDir, fmt.Sprintf("schedule_%s.png", group.GroupName))
	if err := browser.TakeScreenshotHTML(html, imagePath); err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
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
	_, err = api.Send(editConfig)
	return err
}

func OnUpdateTeacher(
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	screenshotDir, templateFile string,
	query *tgbotapi.CallbackQuery,
	args []string,
) error {
	teacherID := args[0]
	teacher, err := repo.GetTeacherByTeacherID(teacherID)
	if err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get teacher by teacher id (%s): %w", teacherID, err)
	}

	scheduleConfig := scraper.TeacherScheduleConfig(teacher)
	schedule, err := scraper.FetchScheduleWithBrowser(repo, browser, scheduleConfig)
	if err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch schedule of teacher (%s): %w", teacherID, err)
	}

	html := schedule.HTML(cache, templateFile)
	imagePath := path.Join(screenshotDir, utils.ScheduleScreenshotFileName(scheduleConfig))
	if err := browser.TakeScreenshotHTML(html, imagePath); err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
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
	_, err = api.Send(editConfig)
	return err
}

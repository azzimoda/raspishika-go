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
	api *tgbotapi.BotAPI, repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache,
	screenshotDir, templateFile string, query *tgbotapi.CallbackQuery, args []string,
) error {
	groupName := args[0]
	group, err := repo.GetGroupByName(groupName)
	if err != nil {
		api.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", groupName, err)
	}

	schedule, err := scraper.FetchGroupSchedule(cache, scraper.GroupScheduleConfig(group))
	if err != nil {
		api.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsFailedFetchSchedule)
		return err
	}

	html := schedule.HTML(cache, templateFile)

	imagePath := path.Join(screenshotDir, fmt.Sprintf("schedule_%s.png", group.GroupName))
	if err := browser.TakeScreenshotHTML(html, imagePath); err != nil {
		api.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
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

package commands

import (
	"fmt"
	"path"

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
		return err
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Msg("Group not set, offering to set group")
		return OnGroup(api, repo, browser, cache, msg)
	}

	return SendWeekSchedule(api, repo, browser, cache, screenshotDir, templateFile, msg.Chat.ID, *chat.GroupName)
}

func SendWeekSchedule(api *tgbotapi.BotAPI, repo *database.Repository, browser *browser.BrowserService,
	cache *cache.Cache, screenshotDir string, templateFile string, chatID int64, groupName string,
) error {
	group, err := repo.GetGroupByName(groupName)
	if err != nil {
		utils.SendErrorMessage(api, chatID, utils.ErrMsgTryLater)
		return err
	}

	schedule, err := scraper.FetchGroupSchedule(cache, scraper.GroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(api, chatID, utils.ErrMsFailedFetchSchedule)
		return err
	}

	html := schedule.HTML(cache, templateFile)
	imagePath := path.Join(screenshotDir, fmt.Sprintf("schedule_%s.png", groupName))
	if err := browser.TakeScreenshotHTML(html, imagePath); err != nil {
		utils.SendErrorMessage(api, chatID, utils.ErrMsgTryLater)
		return err
	}

	newMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(imagePath))
	newMsg.ReplyMarkup = utils.InlineButtonMarkupUpdate(groupName)
	_, err = api.Send(newMsg)
	return err
}

func OnTomorrow(api *tgbotapi.BotAPI, repo *database.Repository, cache *cache.Cache, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnTomorrow")
}

func OnLeft(api *tgbotapi.BotAPI, repo *database.Repository, cache *cache.Cache, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnLeft")
}

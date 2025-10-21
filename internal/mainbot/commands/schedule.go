package commands

import (
	"fmt"
	"os"
	"path"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
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

	group, err := repo.GetGroupByName(*chat.GroupName)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return err
	}

	schedule, err := scraper.FetchGroupSchedule(cache, scraper.NewGroupScheduleConfig(group))
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsFailedFetchSchedule)
		return err
	}
	// log.Trace().Msgf("schedule: %v", schedule)

	html := schedule.HTML(cache, templateFile)

	if log.Logger.GetLevel() <= zerolog.DebugLevel {
		if err := os.MkdirAll("storage/cache/", 0755); err != nil {
			log.Error().Err(err).Msg("Failed to create cache directory")
		}
		filename := "storage/cache/schedule_"+group.GroupName+".html"
		if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
			log.Error().Err(err).Msg("Failed to save schedule HTML")
		}
		log.Debug().Msgf("Saved schedule HTML to %s", filename)
	}

	imagePath := path.Join(screenshotDir, fmt.Sprintf("schedule_%s.png", group.GroupName))
	if err := browser.TakeScreenshotHTML(html, imagePath); err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return err
	}

	newMsg := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FilePath(imagePath))
	newMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Обновить", "update_group\n"+*chat.GroupName)))
	_, err = api.Send(newMsg)
	return err
}

func OnTomorrow(api *tgbotapi.BotAPI, repo *database.Repository, cache *cache.Cache, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnTomorrow")
}

func OnLeft(api *tgbotapi.BotAPI, repo *database.Repository, cache *cache.Cache, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnLeft")
}

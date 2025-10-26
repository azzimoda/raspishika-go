package commands

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// OnQuick sends department selection menu.
func OnQuick(api *tgbotapi.BotAPI, repo *database.Repository, cache *cache.Cache, msg *tgbotapi.Message) error {
	departments, err := scraper.FetchDepartments(cache)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch departments: %w", err)
	}

	if err := repo.UpdateChatState(msg.Chat.ID, database.ChatStateQuickSelectingDepartment); err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Выберите отделение")
	newMsg.ReplyMarkup = departmentSelectionMarkup(departments, true)
	_, err = api.Send(newMsg)
	return err
}

func OnTextQuickGroup(
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	screenshotDir, templateFile string,
	msg *tgbotapi.Message,
) error {
	if err := repo.UpdateChatState(msg.Chat.ID, database.ChatStateDefault); err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	return SendWeekSchedule(api, repo, browser, cache, screenshotDir, templateFile, msg.Chat.ID, msg.Text)
}

func OnTeacher(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: OnTeacher")
}

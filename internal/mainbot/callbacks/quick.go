package callbacks

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"
)

func OnQuickSelectDepartment(
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	query *tgbotapi.CallbackQuery,
	args []string,
) error {
	api.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	groups, err := scraper.FetchDepartmentGroups(repo, browser, cache, args[0])
	if err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch groups: %w", err)
	}

	if err := repo.UpdateChatState(query.Message.Chat.ID, database.ChatStateQuickSelectingGroup); err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(query.Message.Chat.ID, "Выберите группу на клавиатуре")
	newMsg.ReplyMarkup = groupsReplyMarkup(groups)
	_, err = api.Send(newMsg)
	return err
}

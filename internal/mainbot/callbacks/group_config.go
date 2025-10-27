package callbacks

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func OnSelectDepartment(
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

	if err := repo.UpdateChatState(query.Message.Chat.ID, database.ChatStateSelectingGroup); err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(query.Message.Chat.ID, "Выберите группу на клавиатуре")
	newMsg.ReplyMarkup = groupsReplyMarkup(groups)
	_, err = api.Send(newMsg)
	return err
}

func groupsReplyMarkup(groups []database.Group) tgbotapi.ReplyKeyboardMarkup {
	rows := make([][]tgbotapi.KeyboardButton, 0)

	rows = append(rows, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Отмена")))
	for i := 0; i < len(groups); i += 2 {
		row := make([]tgbotapi.KeyboardButton, 0)
		for j := i; j < len(groups) && j < i+2; j++ {
			row = append(row, tgbotapi.KeyboardButton{Text: groups[j].GroupName})
		}
		rows = append(rows, row)
	}

	return tgbotapi.ReplyKeyboardMarkup{
		Keyboard: rows,
		ResizeKeyboard: true,
		OneTimeKeyboard: true,
		Selective: true,
	}
}

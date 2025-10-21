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

func OnSettings(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnSettings")
}

// OnGroup sends department selection menu.
func OnGroup(
	api *tgbotapi.BotAPI, repo *database.Repository, browser *browser.BrowserService, cache *cache.Cache, msg *tgbotapi.Message,
) error {
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return err
	}

	departments, err := scraper.FetchDepartments(cache)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch departments: %w", err)
	}

	currentGroup := "Группа не выбрана"
	if chat.GroupName != nil && *chat.GroupName != "" {
		currentGroup = fmt.Sprintf("Текущая группа: %s", *chat.GroupName)
	}
	text := fmt.Sprintf("%s\n\nВыберите отделение", currentGroup)

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ReplyMarkup = departmentSelectionMarkup(departments)
	_, err = api.Send(newMsg)
	return err
}

func departmentSelectionMarkup(departments []scraper.Department) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0)

	for i := 0; i < len(departments); i += 2 {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		for j := i; j < len(departments) && j < i+2; j++ {
			row = append(row,
				tgbotapi.NewInlineKeyboardButtonData(
					departments[j].Name, fmt.Sprintf("select_department\n%s", departments[j].Name),
				),
			)
		}
		rows = append(rows, row)
	}
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Отмена", "delete"),
	})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func OnTextGroup(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message, chat *database.Chat) error {
	group, err := repo.GetGroupByName(msg.Text)
	if err != nil {
		return err
	}

	chat.State = database.ChatStateDefault
	chat.GroupName = &group.GroupName
	chat.DepartmentName = &group.DepartmentName
	if err := repo.UpdateChat(chat); err != nil {
		return err
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Теперь вы в группе %s", group.GroupName))
	newMsg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true}
	_, err = api.Send(newMsg)
	return err
}

func OnDailyTime(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnDailyTime")
}

func OnDailyOff(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnDailyOff")
}

func OnReminder(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message, isOn bool) error {
	return fmt.Errorf("Unimplemented: commands.OnReminder")
}

func OnAccess(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnAccess")
}

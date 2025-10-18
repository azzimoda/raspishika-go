package commands

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func OnSettings(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnSettings")
}

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
		return fmt.Errorf("failed to fetch departments: %v", err)
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

	log.Error().Msg("Unimplemented: departmentSelectionMarkup")

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
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

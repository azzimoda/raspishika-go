package commands

import (
	"fmt"
	"time"

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
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	msg *tgbotapi.Message,
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

	if err := repo.UpdateChatState(msg.Chat.ID, database.ChatStateSelectingDepartment); err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	currentGroup := "Группа не выбрана"
	if chat.GroupName != nil && *chat.GroupName != "" {
		currentGroup = fmt.Sprintf("Текущая группа: %s", *chat.GroupName)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("%s\n\nВыберите отделение", currentGroup))
	newMsg.ReplyMarkup = departmentSelectionMarkup(departments, false)
	_, err = api.Send(newMsg)
	return err
}

func departmentSelectionMarkup(departments []scraper.Department, isQuick bool) tgbotapi.InlineKeyboardMarkup {
	command := "select_department"
	if isQuick {
		command = "quick_select_department"
	}

	rows := make([][]tgbotapi.InlineKeyboardButton, 0)

	for i := 0; i < len(departments); i += 2 {
		row := make([]tgbotapi.InlineKeyboardButton, 0)
		for j := i; j < len(departments) && j < i+2; j++ {
			row = append(row,
				tgbotapi.NewInlineKeyboardButtonData(
					departments[j].Name, fmt.Sprintf("%s\n%s", command, departments[j].Name),
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
	api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	group, err := repo.GetGroupByName(msg.Text)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", msg.Text, err)
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
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		return fmt.Errorf("failed to get chat by chat ID (%d): %w", msg.Chat.ID, err)
	}

	if err := repo.UpdateChatState(msg.Chat.ID, database.ChatStateSelectingTime); err != nil {
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	time := ""
	if chat.DailySendingTime == "" {
		time = "Время не установлено"
	} else {
		time = "Установленное время: " + chat.DailySendingTime
	}
	text := fmt.Sprintf("_%s_\nПришлите желаемое время рассылки, например `19:00`", time)

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Отмена", "delete")))

	_, err = api.Send(newMsg)
	return err
}

func OnTextTime(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message, chat *database.Chat) error {
	api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	t, err := time.Parse("15:04", msg.Text)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, "Неправильный вормат времени, попробуйте ещё раз: `19:00`")
		return nil
	}
	timeStr := t.Format("15:04")

	chat.State = database.ChatStateDefault
	chat.DailySendingTime = timeStr
	if err := repo.UpdateChat(chat); err != nil {
		return fmt.Errorf("failed to update chat: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Время рассылки установлено на "+timeStr)
	_, err = api.Send(newMsg)
	return err
}

func OnDailyOff(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	chat.DailySendingTime = ""
	if err := repo.UpdateChat(chat); err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat data: %w", err)
	}

	_, err = api.Send(tgbotapi.NewMessage(msg.Chat.ID, "Ежедневная рассылка выключена"))
	return err
}

func OnReminder(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message, isOn bool) error {
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d) %w", msg.Chat.ID, err)
	}

	chat.PairSending = isOn
	if err := repo.UpdateChat(chat); err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat data: %w", err)
	}

	text := "Напоминания выключены"
	if isOn {
		text = "Напоминания включены"
	}
	_, err = api.Send(tgbotapi.NewMessage(msg.Chat.ID, text))
	return err
}

func OnAccess(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	return fmt.Errorf("Unimplemented: commands.OnAccess")
}

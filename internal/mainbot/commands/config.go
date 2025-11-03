package commands

import (
	"fmt"
	"time"

	"github.com/azzimoda/raspishika-go/internal/database"
	botutils "github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (ch *CommandHandler) OnSettings(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat ID (%d): %w", msg.Chat.ID, err)
	}

	return SendSettingsMenu(ch.Bot.API(), chat, msg.Chat.ID)
}

func SendSettingsMenu(api *tgbotapi.BotAPI, chat *database.Chat, tgChatID int64) error {
	dailyTime := chat.DailySendingTime
	if chat.DailySendingTime == "" {
		dailyTime = "выключено"
	}
	pairNotification := "выключено"
	if chat.PairSending {
		pairNotification = "включено"
	}

	text := fmt.Sprintf(`Меню настроек

Группа: %s
Ежедневная рассылка: %s
Напоминания перед парами: %s`,
		utils.DerefOrTypeDefault(chat.GroupName),
		dailyTime,
		pairNotification,
	)
	if !chat.IsPrivate() {
		text += fmt.Sprintf("\nУровень доступа: %d", chat.Access)
	}

	newMsg := tgbotapi.NewMessage(tgChatID, text)
	newMsg.ReplyMarkup = settingsInlineMarkup(chat)
	_, err := api.Send(newMsg)
	return err
}

func settingsInlineMarkup(chat *database.Chat) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Изменить группу", "config_group"),
	})

	if chat.DailySendingTime == "" {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Включить ежедневную рассылку", "config_daily_time"),
		})
	} else {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Изменить время рассылки", "config_daily_time"),
			tgbotapi.NewInlineKeyboardButtonData("Выключить ежедневную рассылку", "daily_off"),
		})
	}

	if chat.PairSending {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Выключить напоминания", "config_reminder\nfalse"),
		})
	} else {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Включить напоминания", "config_reminder\ntrue"),
		})
	}

	if !chat.IsPrivate() {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("0", "config_access\n0"),
			tgbotapi.NewInlineKeyboardButtonData("1", "config_access\n1"),
			tgbotapi.NewInlineKeyboardButtonData("2", "config_access\n2"),
		})
		for i := range 3 {
			if i == chat.Access {
				rows[len(rows)-1][i].Text = fmt.Sprintf("[%d]", i)
			}
		}
	}

	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Закрыть", "delete"),
	})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// OnGroup sends department selection menu.
func (ch *CommandHandler) OnGroup(
	msg *tgbotapi.Message,
) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return err
	}

	departments, err := scraper.FetchDepartments(ch.Bot.Cache())
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch departments: %w", err)
	}

	if err := ch.Bot.Repo().UpdateChatState(msg.Chat.ID, database.ChatStateSelectingDepartment); err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	currentGroup := "Группа не выбрана"
	if chat.GroupName != nil && *chat.GroupName != "" {
		currentGroup = fmt.Sprintf("Текущая группа: %s", *chat.GroupName)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("%s\n\nВыберите отделение", currentGroup))
	newMsg.ReplyMarkup = departmentSelectionMarkup(departments, false)
	_, err = ch.Bot.API().Send(newMsg)
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

func (ch *CommandHandler) OnTextGroup(msg *tgbotapi.Message, chat *database.Chat) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	group, err := ch.Bot.Repo().GetGroupByName(msg.Text)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name (%s): %w", msg.Text, err)
	}

	chat.State = database.ChatStateDefault
	chat.GroupName = &group.GroupName
	chat.DepartmentName = &group.DepartmentName
	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		return err
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Теперь вы в группе %s", group.GroupName))
	newMsg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true}
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func (ch *CommandHandler) OnDailyTime(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		return fmt.Errorf("failed to get chat by chat ID (%d): %w", msg.Chat.ID, err)
	}

	if err := ch.Bot.Repo().UpdateChatState(msg.Chat.ID, database.ChatStateSelectingTime); err != nil {
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

	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func (ch *CommandHandler) OnTextTime(msg *tgbotapi.Message, chat *database.Chat) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	t, err := time.Parse("15:04", msg.Text)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, "Неправильный вормат времени, попробуйте ещё раз: `19:00`")
		return nil
	}
	timeStr := t.Format("15:04")

	chat.State = database.ChatStateDefault
	chat.DailySendingTime = timeStr
	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		return fmt.Errorf("failed to update chat: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Время рассылки установлено на "+timeStr)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func (ch *CommandHandler) OnDailyOff(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	chat.DailySendingTime = ""
	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat data: %w", err)
	}

	_, err = ch.Bot.API().Send(tgbotapi.NewMessage(msg.Chat.ID, "Ежедневная рассылка выключена"))
	return err
}

func (ch *CommandHandler) OnReminder(msg *tgbotapi.Message, isOn bool) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d) %w", msg.Chat.ID, err)
	}

	chat.PairSending = isOn
	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat data: %w", err)
	}

	text := "Напоминания выключены"
	if isOn {
		text = "Напоминания включены"
	}
	_, err = ch.Bot.API().Send(tgbotapi.NewMessage(msg.Chat.ID, text))
	return err
}

func (ch *CommandHandler) OnAccess(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	newMsg := tgbotapi.NewMessage(
		msg.Chat.ID,
		fmt.Sprintf(`Текущий уровень доступа: %d
		0 — без ограничений
		1 — настройки только для админов
		2 — все команды только для админов`, chat.Access),
	)
	newMsg.ReplyMarkup = botutils.AccessMenuInlineMarkup(chat.Access)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

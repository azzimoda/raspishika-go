package commands

import (
	"fmt"
	"strings"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
	"github.com/schollz/closestmatch"
)

// OnQuick sends department selection menu.
func (ch *CommandHandler) OnQuick(msg *tgbotapi.Message) error {
	departments, err := scraper.FetchDepartments(ch.Bot.Cache())
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch departments: %w", err)
	}

	if err := ch.Bot.Repo().UpdateChatState(msg.Chat.ID, database.ChatStateQuickSelectingDepartment); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Выберите отделение")
	newMsg.ReplyMarkup = departmentSelectionMarkup(departments, true)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func (ch *CommandHandler) OnTextQuickGroup(msg *tgbotapi.Message) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat ID (%d): %w", msg.Chat.ID, err)
	}

	if err := ch.Bot.Repo().UpdateChatState(msg.Chat.ID, database.ChatStateDefault); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	group, err := ch.Bot.Repo().GetGroupByName(msg.Text)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name %s: %w", msg.Text, err)
	}
	scheduleCfg := scraper.GroupScheduleConfig(group)
	return ch.SendWeekSchedule(chat, scheduleCfg)
}

func (ch *CommandHandler) OnTeacher(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat ID (%d): %w", msg.Chat.ID, err)
	}

	teachers, err := ch.Bot.Repo().GetTeacherByChatID(chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get teachers by recent teachers: %w", err)
	}

	if err := ch.Bot.Repo().UpdateChatState(msg.Chat.ID, database.ChatStateSelectingTeacher); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Пришлите полное имя преподавателя или его часть")
	newMsg.ReplyMarkup = recentTeachersInlineMarkup(teachers)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func recentTeachersInlineMarkup(teachers []database.Teacher) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0)

	for _, teacher := range teachers {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			teacher.Name, "select_teacher\n"+teacher.TeacherID)))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Отмена", "delete")))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (ch *CommandHandler) OnTextTeacherName(msg *tgbotapi.Message) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))

	teachers, err := scraper.FetchTeachers(ch.Bot.Repo(), ch.Bot.Browser())
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch teachers: %w", err)
	}
	log.Trace().Int("teachers", len(teachers)).Msg("Fetched teachers")

	names := make([]string, len(teachers))
	for i, t := range teachers {
		names[i] = t.Name
	}
	matchedNames := selectTeachers(names, msg.Text)
	ids := make([]string, len(matchedNames))
	for i, name := range matchedNames {
		for _, t := range teachers {
			if t.Name == name {
				ids[i] = t.TeacherID
			}
		}
	}

	if len(matchedNames) == 0 {
		// Maybe this case is impossible.
		newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Не удалось найти преподавателя, попробуйте снова")
		_, err := ch.Bot.API().Send(newMsg)
		return err
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Выберите проподавателя из списка или попробуйте снова")
	newMsg.ReplyMarkup = inlineButtonMarkupTeachers(matchedNames, ids)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func selectTeachers(teachers []string, name string) []string {
	for _, t := range teachers {
		if strings.EqualFold(t, name) {
			return []string{t}
		}
	}
	return closestmatch.New(teachers, []int{2, 3, 4}).ClosestN(name, 5)
}

func inlineButtonMarkupTeachers(names, ids []string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	for i, name := range names {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(name, "select_teacher\n"+ids[i])))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Отмена", "delete")))

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

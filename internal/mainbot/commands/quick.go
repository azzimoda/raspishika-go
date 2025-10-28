package commands

import (
	"fmt"
	"strings"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/rs/zerolog/log"
	"github.com/schollz/closestmatch"

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

	group, err := repo.GetGroupByName(msg.Text)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get group by name %s: %w", msg.Text, err)
	}
	scheduleCfg := scraper.GroupScheduleConfig(group)
	return SendWeekSchedule(api, repo, browser, cache, screenshotDir, templateFile, msg.Chat.ID, scheduleCfg)
}

func OnTeacher(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat ID (%d): %w", msg.Chat.ID, err)
	}

	teachers, err := repo.GetTeacherByChatID(chat.ID)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get teachers by recent teachers: %w", err)
	}

	if err := repo.UpdateChatState(msg.Chat.ID, database.ChatStateSelectingTeacher); err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Пришлите полное имя преподавателя или его часть")
	newMsg.ReplyMarkup = recentTeachersInlineMarkup(teachers)
	_, err = api.Send(newMsg)
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

func OnTextTeacherName(
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	msg *tgbotapi.Message,
) error {
	teachers, err := scraper.FetchTeachers(repo, browser)
	if err != nil {
		utils.SendErrorMessage(api, msg.Chat.ID, utils.ErrMsgTryLater)
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
		_, err := api.Send(newMsg)
		return err
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Выберите проподавателя из списка или попробуйте снова")
	newMsg.ReplyMarkup = inlineButtonMarkupTeachers(matchedNames, ids)
	_, err = api.Send(newMsg)
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

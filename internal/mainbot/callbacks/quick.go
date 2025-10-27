package callbacks

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"
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

func OnSelectTeacher(
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	screenshotDir, templateFile string,
	query *tgbotapi.CallbackQuery,
	args []string,
) error {
	api.Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	teachers, err := scraper.FetchTeachers(repo, browser)
	if err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch teachers: %w", err)
	}
	var teacher *database.Teacher
	for _, t := range teachers {
		if t.TeacherID == args[0] {
			teacher = &t
			break
		}
	}
	if teacher == nil {
		// NOTE: This code supposed to be unreachable, because teacher ID from callback query is taken from database.
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to find teacher by given teacher ID (%s)", args[0])
	}

	if err := repo.UpdateChatState(query.Message.Chat.ID, database.ChatStateDefault); err != nil {
		utils.SendErrorMessage(api, query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	scheduleCfg := scraper.TeacherScheduleConfig(teacher)
	return commands.SendWeekSchedule(api, repo, browser, cache, screenshotDir, templateFile, query.Message.Chat.ID,
		scheduleCfg)
}

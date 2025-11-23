package mainbot

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
)

const (
	ErrMsgTryLater             = "Произошла ошибка, попробуйте позже"
	ErrMsgCouldNotLoadSchedule = "Не удалось загрузить расписание, попробуйте позже"
	ErrMsgCouldNotUpdateData   = "Не удалось обновить данные, попробуйте позже"
	ErrMsgSelectGroupAgain     = "Не удалось найти группу, выберите группу ещё раз"
)

func (mb *MainBot) groupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Group handler")

	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
	})
	addContextHandlerError(ctx, err)

	mb.sendGroupMenu(ctx, b)
}

func (mb *MainBot) sendGroupMenu(ctx context.Context, b *bot.Bot) {
	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	chatID := chat.TgChatID

	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgTryLater})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(chatID).
			Msg("Error in groupHandler")
		return
	}

	departments, err := scraper.FetchDepartments(mb.services.Cache)
	if err != nil {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgTryLater})
		addContextHandlerError(ctx, err)
		return
	}

	chat.State = database.ChatStateSelectingDepartment
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgTryLater})
		addContextHandlerError(ctx, err)
		return
	}

	currentGroup := "Группа не выбрана"
	if chat.GroupName != nil {
		currentGroup = fmt.Sprintf("Текущая группа: %s", *chat.GroupName)
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("%s\n\nВыберите отделение", currentGroup),
		ReplyMarkup: departmentSelectionMarkup(departments, false),
	})
	addContextHandlerError(ctx, err)
}

func departmentSelectionMarkup(departments []scraper.Department, isQuick bool) models.InlineKeyboardMarkup {
	command := "select_department"
	if isQuick {
		command = "quick_select_department"
	}

	keyboard := make([][]models.InlineKeyboardButton, 0)
	for i := 0; i < len(departments); i += 2 {
		row := make([]models.InlineKeyboardButton, 0)
		for j := i; j < len(departments) && j < i+2; j++ {
			row = append(row, models.InlineKeyboardButton{Text: departments[j].Name,
				CallbackData: fmt.Sprintf("%s\n%s", command, departments[j].Name)})
		}
		keyboard = append(keyboard, row)
	}
	keyboard = append(keyboard, []models.InlineKeyboardButton{{Text: "Отмена", CallbackData: "delete"}})
	return models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func (mb *MainBot) selectDepartmentHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Select department handler")

	callbackCommand := tgbothelpers.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: message.Chat.ID, MessageID: message.ID})
	addContextHandlerError(ctx, err)

	groups, err := scraper.FetchDepartmentGroups(
		mb.services.Repo,
		mb.services.Browser,
		mb.services.Cache,
		callbackCommand.Arg(0),
	)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgTryLater})
	}

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgTryLater})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(message.Chat.ID).
			Msg("Error in selectDepartmentHandler")
		return
	}

	chat.State = database.ChatStateSelectingGroup
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      message.Chat.ID,
		Text:        "Выберите группу на клавиатуре",
		ReplyMarkup: groupsReplyMarkup(groups),
	})
	addContextHandlerError(ctx, err)
}

func groupsReplyMarkup(groups []database.Group) models.ReplyKeyboardMarkup {
	keyboard := [][]models.KeyboardButton{{{Text: "Назад"}}}
	for i := 0; i < len(groups); i += 2 {
		row := make([]models.KeyboardButton, 0)
		for j := i; j < len(groups) && j < i+2; j++ {
			row = append(row, models.KeyboardButton{Text: groups[j].GroupName})
		}
		keyboard = append(keyboard, row)
	}
	return models.ReplyKeyboardMarkup{Keyboard: keyboard, ResizeKeyboard: true, OneTimeKeyboard: true}
}

func (mb *MainBot) textGroupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Text group handler")

	group, err := mb.FetchGroupByNameWithValidation(update.Message.Text)
	if errors.Is(err, ErrWrongGroupNameFormat) {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Неправильный формат группы, попробуйте ещё раз",
		})
		return
	} else if errors.Is(err, ErrGroupNotFound) {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Группа не найдена, попробуйте ещё раз",
		})
		return
	} else if err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to try get group: %w", err))
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgTryLater})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(update.Message.Chat.ID).
			Msg("Error in textGroupHandler")
		return
	}
	chat.State = database.ChatStateDefault
	chat.GroupName = &group.GroupName
	chat.DepartmentName = &group.DepartmentName
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to update chat: %w", err))
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgCouldNotUpdateData})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        fmt.Sprintf("Теперь вы в группе %s", group.GroupName),
		ReplyMarkup: mainMenuReplyMarkup(chat.IsPrivate()),
	})
	addContextHandlerError(ctx, err)
}

var (
	ErrWrongGroupNameFormat = errors.New("wrong group name format")
	ErrGroupNotFound        = errors.New("group not found")
)

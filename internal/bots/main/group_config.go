package mainbot

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

const (
	ErrMsgTryLater             = "Произошла ошибка, попробуйте позже"
	ErrMsgCouldNotLoadSchedule = "Не удалось загрузить расписание, попробуйте позже"
	ErrMsgCouldNotUpdateData   = "Не удалось обновить данные, попробуйте позже"
	ErrMsgCouldNotSendSchedule = "Не удалось отправить расписание, попробуте позже"
	ErrMsgSelectGroupAgain     = "Не удалось найти группу, выберите группу ещё раз"
)

func (mb *MainBot) groupHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Group handler")

	_, err := bothelpers.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	mb.sendGroupMenu(ctx, b, update.Message.MessageThreadID)
}

func (mb *MainBot) sendGroupMenu(ctx context.Context, b *bot.Bot, messageThreadID int) {
	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	chatID := chat.TgChatID

	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: messageThreadID,
			Text:            ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(chatID).
			Msg("Error in groupHandler")
		return
	}

	departments, err := scraper.FetchDepartments(mb.services.Repo)
	if err != nil {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: messageThreadID,
			Text:            ErrMsgTryLater,
		})
		addContextHandlerError(ctx, err)
		return
	}

	chat.State = models.ChatStateSelectingDepartment
	if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: messageThreadID,
			Text:            ErrMsgTryLater,
		})
		addContextHandlerError(ctx, err)
		return
	}

	currentGroup := "Группа не выбрана"
	if chat.GroupName != nil {
		currentGroup = fmt.Sprintf("Текущая группа: %s", *chat.GroupName)
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: messageThreadID,
		Text:            fmt.Sprintf("%s\n\nВыберите отделение", currentGroup),
		ReplyMarkup:     departmentSelectionMarkup(departments),
	})
	addContextHandlerError(ctx, err)
}

func departmentSelectionMarkup(departments []models.Department) tgmodels.InlineKeyboardMarkup {
	keyboard := make([][]tgmodels.InlineKeyboardButton, 0)
	for i := 0; i < len(departments); i += 2 {
		row := make([]tgmodels.InlineKeyboardButton, 0)
		for j := i; j < len(departments) && j < i+2; j++ {
			row = append(row, tgmodels.InlineKeyboardButton{Text: departments[j].Name,
				CallbackData: fmt.Sprintf("%s\n%s", CallbackCommandSelectDepartment, departments[j].Name)})
		}
		keyboard = append(keyboard, row)
	}

	keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
		{Text: "Отмена", CallbackData: CallbackCommandDeleteConfig},
	})
	return tgmodels.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func (mb *MainBot) selectDepartmentHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Select department handler")

	callbackCommand := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

	_, err := bothelpers.DeleteMessageSafely(ctx, b, message)
	addContextHandlerError(ctx, err)

	groups, err := scraper.FetchDepartmentGroups(mb.services.Repo, mb.services.Browser, callbackCommand.Arg(0))
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
	}

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(message.Chat.ID).
			Msg("Error in selectDepartmentHandler")
		return
	}

	chat.State = models.ChatStateSelectingGroup
	if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			Text:            ErrMsgTryLater,
			MessageThreadID: message.MessageThreadID,
		})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          message.Chat.ID,
		MessageThreadID: message.MessageThreadID,
		Text:            "Выберите группу на клавиатуре или введите название в верном формате (например: ИСПт-22-(9)-2)",
		ReplyMarkup:     groupsReplyMarkup(groups),
	})
	addContextHandlerError(ctx, err)
}

func groupsReplyMarkup(groups []models.Group) tgmodels.ReplyKeyboardMarkup {
	keyboard := [][]tgmodels.KeyboardButton{{{Text: "Отмена"}}}
	for i := 0; i < len(groups); i += 2 {
		row := make([]tgmodels.KeyboardButton, 0)
		for j := i; j < len(groups) && j < i+2; j++ {
			row = append(row, tgmodels.KeyboardButton{Text: groups[j].GroupName})
		}
		keyboard = append(keyboard, row)
	}
	return tgmodels.ReplyKeyboardMarkup{
		Keyboard:        keyboard,
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
		Selective:       true,
	}
}

func (mb *MainBot) textGroupHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Text group handler")

	group, err := mb.FetchGroupByNameWithValidation(update.Message.Text)
	if errors.Is(err, ErrWrongGroupNameFormat) {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Неправильный формат группы, попробуйте ещё раз",
		})
		return
	} else if errors.Is(err, ErrGroupNotFound) {
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Группа не найдена, попробуйте ещё раз",
		})
		return
	} else if err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to try get group: %w", err))
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(update.Message.Chat.ID).
			Msg("Error in textGroupHandler")
		return
	}
	chat.State = models.ChatStateDefault
	chat.GroupName = &group.GroupName
	chat.DepartmentName = &group.DepartmentName
	if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to update chat: %w", err))
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("Теперь вы в группе %s", group.GroupName),
		ReplyMarkup:     mainMenuReplyMarkup(chat.IsPrivate()),
	})
	addContextHandlerError(ctx, err)
}

var (
	ErrWrongGroupNameFormat = errors.New("wrong group name format")
	ErrGroupNotFound        = errors.New("group not found")
)

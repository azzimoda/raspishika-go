package mainbot

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/botutil"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/service/scraper"
	"github.com/azzimoda/raspishika-go/pkg/bothelper"
)

func (mb *MainBot) groupHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Group handler")

	_, err := bothelper.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	mb.sendGroupMenu(ctx, b, update.Message.MessageThreadID)
}

func (mb *MainBot) sendGroupMenu(ctx context.Context, b *bot.Bot, messageThreadID int) {
	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	chatID := chat.TgChatID

	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: messageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(chatID).
			Msg("Error in groupHandler")
		return
	}

	departments, err := scraper.FetchDepartments(mb.container.Group)
	if err != nil {
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: messageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		addContextHandlerError(ctx, err)
		return
	}

	chat.State = model.ChatStateSelectingDepartment
	if err := mb.container.Chat.Update(chat); err != nil {
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: messageThreadID,
			Text:            botutil.ErrMsgTryLater,
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

func departmentSelectionMarkup(departments []model.Department) tgmodels.InlineKeyboardMarkup {
	keyboard := make([][]tgmodels.InlineKeyboardButton, 0)
	for i := 0; i < len(departments); i += 2 {
		row := make([]tgmodels.InlineKeyboardButton, 0)
		for j := i; j < len(departments) && j < i+2; j++ {
			row = append(row, tgmodels.InlineKeyboardButton{Text: departments[j].Name.String(),
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

	callbackCommand := bothelper.ParseCallbackData(update.CallbackQuery.Data)
	message := update.CallbackQuery.Message.Message

	_, err := bothelper.DeleteMessageSafely(ctx, b, message)
	addContextHandlerError(ctx, err)

	groups, err := scraper.FetchDepartmentGroups(mb.container.Group, mb.services.Browser,
		model.DepartmentName(callbackCommand.Arg(0)))
	if err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
	}

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			MessageThreadID: message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(message.Chat.ID).
			Msg("Error in selectDepartmentHandler")
		return
	}

	chat.State = model.ChatStateSelectingGroup
	if err := mb.container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          message.Chat.ID,
			Text:            botutil.ErrMsgTryLater,
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

func groupsReplyMarkup(groups []model.Group) tgmodels.ReplyKeyboardMarkup {
	keyboard := [][]tgmodels.KeyboardButton{{{Text: "Отмена"}}}
	for i := 0; i < len(groups); i += 2 {
		row := make([]tgmodels.KeyboardButton, 0)
		for j := i; j < len(groups) && j < i+2; j++ {
			row = append(row, tgmodels.KeyboardButton{Text: groups[j].GroupName.String()})
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

	group, err := scraper.FetchGroupByNameWithValidation(
		mb.container.Group,
		mb.services.Browser,
		model.GroupName(update.Message.Text),
	)
	if errors.Is(err, scraper.ErrWrongGroupNameFormat) {
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Неправильный формат группы, попробуйте ещё раз",
		})
		return
	} else if errors.Is(err, scraper.ErrGroupNotFound) {
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Группа не найдена, попробуйте ещё раз",
		})
		return
	} else if err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to try get group: %w", err))
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		return
	}

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(update.Message.Chat.ID).
			Msg("Error in textGroupHandler")
		return
	}
	chat.State = model.ChatStateDefault
	chat.GroupName = &group.GroupName
	chat.DepartmentName = &group.DepartmentName
	if err := mb.container.Chat.Update(chat); err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to update chat: %w", err))
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotUpdateData,
		})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            fmt.Sprintf("Теперь вы в группе %s", group.GroupName),
		ReplyMarkup:     botutil.MainMenuReplyMarkup(chat.IsPrivate()),
	})
	addContextHandlerError(ctx, err)
}

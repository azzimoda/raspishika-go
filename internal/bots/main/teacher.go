package mainbot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (mb *MainBot) teacherHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Teacher handler")

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgTryLater})
		return
	}

	teachers, err := models.GetTeacherByChatID(mb.services.Repo.DB, chat.ID)
	if err != nil {
		addContextHandlerError(ctx, err)
		teachers = []models.Teacher{}
	}

	if err := models.UpdateChatState(mb.services.Repo.DB, chat.TgChatID, models.ChatStateSelectingTeacher); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgCouldNotUpdateData})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Пришлите полное имя преподавателя или его часть",
		ReplyMarkup:     teacherInlineMarkup(teachers),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) textTeacherNameHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Text teacher name handler")

	_, err := bothelpers.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	// Search for the teacher in the database.
	teachers, err := scraper.FetchTeachers(mb.services.Repo, mb.services.Browser)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	names := make([]string, len(teachers))
	for i, t := range teachers {
		names[i] = t.Name.String()
	}
	matchedNames := utils.MatchStrings(names, update.Message.Text, 5)
	matchedTeachers := make([]models.Teacher, len(matchedNames))
	for i, name := range matchedNames {
		for _, t := range teachers {
			if t.Name.String() == name {
				matchedTeachers[i] = t
				break
			}
		}
	}

	if len(matchedTeachers) == 1 {
		// Try to send schedule for the selected teacher.
		chat, ok := ctx.Value(chatContextKey).(*models.Chat)
		if !ok {
			addContextHandlerError(ctx, fmt.Errorf("could not send teacher schedule: %w", ErrNoChatContext))
			// If failed, reask user to select teacher manually.
		} else {
			mb.sendTeacherSchedule(ctx, b, update.Message.MessageThreadID, &update.Message.Chat, chat, &matchedTeachers[0])
			return
		}
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Выберите проподавателя из списка или попробуйте снова",
		ReplyMarkup:     teachersInlineMarkup(matchedTeachers),
	})
}

func teachersInlineMarkup(teachers []models.Teacher) tgmodels.InlineKeyboardMarkup {
	keyboard := make([][]tgmodels.InlineKeyboardButton, 0)
	for _, teacher := range teachers {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{{
			Text:         teacher.Name.String(),
			CallbackData: fmt.Sprintf("%s\n%s", CallbackCommandSelectTeacher, teacher.TeacherID),
		}})
	}
	keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{
		{Text: "Закрыть", CallbackData: CallbackCommandDelete},
	})
	return tgmodels.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func teacherInlineMarkup(teachers []models.Teacher) tgmodels.InlineKeyboardMarkup {
	keyboard := make([][]tgmodels.InlineKeyboardButton, 0)
	for _, teacher := range teachers {
		keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{{
			Text:         teacher.Name.String(),
			CallbackData: fmt.Sprintf("%s\n%s", CallbackCommandSelectTeacher, teacher.TeacherID),
		}})
	}
	keyboard = append(keyboard, []tgmodels.InlineKeyboardButton{{Text: "Отмена", CallbackData: CallbackCommandDelete}})
	return tgmodels.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func (mb *MainBot) selectTeacherHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Select teacher handler")
	message := update.CallbackQuery.Message.Message

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)

	_, err := bothelpers.DeleteMessageSafely(ctx, b, message)
	addContextHandlerError(ctx, err)

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.CallbackQuery.Message.Message.Chat.ID,
			Text:   ErrMsgTryLater,
		})
		return
	}

	teacher, err := models.GetTeacherByTeacherID(mb.services.Repo.DB, command.Arg(0))
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: message.Chat.ID,
			Text:   ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	mb.sendTeacherSchedule(ctx, b, message.MessageThreadID, &message.Chat, chat, teacher)
}

func (mb *MainBot) sendTeacherSchedule(
	ctx context.Context,
	b *bot.Bot,
	messageThreadID int,
	chat *tgmodels.Chat,
	localChat *models.Chat,
	teacher *models.Teacher,
) {
	err := models.AddChatRecentTeacher(mb.services.Repo.DB, localChat.ID, teacher.ID)
	if err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to add recent teacher: %w", err))
	}

	if err := models.UpdateChatState(mb.services.Repo.DB, localChat.TgChatID, models.ChatStateDefault); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chat.ID, Text: ErrMsgCouldNotUpdateData})
	}

	_, err = b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID:          chat.ID,
		MessageThreadID: messageThreadID,
		Action:          tgmodels.ChatActionTyping,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send chat action")
	}

	conf := models.TeacherScheduleConfig(teacher)
	imageFilename, imageData, err := mb.PrepareScheduleImage(conf)
	if err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed preparing schedule data: %w", err))
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chat.ID, Text: ErrMsgCouldNotLoadSchedule})
		return
	}

	err = mb.SendWeekScheduleMessages(ctx, b, messageThreadID, localChat, conf, imageFilename, imageData)
	addContextHandlerError(ctx, err)
}

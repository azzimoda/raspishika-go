package mainbot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
)

func (mb *MainBot) quickHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Quick handler")

	departments, err := scraper.FetchDepartments(mb.services.Cache)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	if err := mb.services.Repo.UpdateChatState(update.Message.Chat.ID, database.ChatStateQuickSelectingDepartment); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "Выберите отделение",
		ReplyMarkup: departmentSelectionMarkup(departments, true),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) quickSelectDepartmentHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Quick select department handler")

	command := tgbothelpers.ParseCallbackData(update.CallbackQuery.Data)

	chatID := update.CallbackQuery.Message.Message.Chat.ID
	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: update.CallbackQuery.Message.Message.ID,
	})
	addContextHandlerError(ctx, err)

	groups, err := scraper.FetchDepartmentGroups(
		mb.services.Repo,
		mb.services.Browser,
		mb.services.Cache,
		command.Arg(0),
	)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	if err := mb.services.Repo.UpdateChatState(chatID, database.ChatStateQuickSelectingGroup); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgCouldNotUpdateData})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Выберите группу на клавиатуре",
		ReplyMarkup: groupsReplyMarkup(groups),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) textQuickGroupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Quick select course handler")

	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
	})
	addContextHandlerError(ctx, err)

	if err := mb.services.Repo.UpdateChatState(update.Message.Chat.ID, database.ChatStateDefault); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   ErrMsgCouldNotUpdateData,
		})
		return
	}

	group, err := mb.services.Repo.GetGroupByName(update.Message.Text)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: "Группа не найдена"})
	}

	err = mb.SendWeekSchedule(ctx, b, update.Message.Chat.ID, update.Message.Chat.Type == models.ChatTypePrivate, scraper.GroupScheduleConfig(group))
	addContextHandlerError(ctx, err)
}

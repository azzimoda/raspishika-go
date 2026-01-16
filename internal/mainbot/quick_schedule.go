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
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	if err := mb.services.Repo.UpdateChatState(update.Message.Chat.ID, database.ChatStateQuickSelectingDepartment); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Выберите отделение",
		ReplyMarkup:     departmentSelectionMarkup(departments, true),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) quickSelectDepartmentHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Quick select department handler")

	command := tgbothelpers.ParseCallbackData(update.CallbackQuery.Data)
	chatID := update.CallbackQuery.Message.Message.Chat.ID

	_, err := tgbothelpers.DeleteMessageSafely(ctx, b, update.CallbackQuery.Message.Message)
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
			ChatID:          chatID,
			MessageThreadID: update.CallbackQuery.Message.Message.MessageThreadID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	if err := mb.services.Repo.UpdateChatState(chatID, database.ChatStateQuickSelectingGroup); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: update.CallbackQuery.Message.Message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.CallbackQuery.Message.Message.Chat.ID,
		MessageThreadID: update.CallbackQuery.Message.Message.MessageThreadID,
		Text:            "Выберите группу на клавиатуре или введите название в верном формате (например: ИСПт-22-(9)-2)",
		ReplyMarkup:     groupsReplyMarkup(groups),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) textQuickGroupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Quick select course handler")

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return
	}

	_, err := tgbothelpers.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	if err := mb.services.Repo.UpdateChatState(update.Message.Chat.ID, database.ChatStateDefault); err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		return
	}

	group, err := mb.services.Repo.GetGroupByName(update.Message.Text)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Группа не найдена",
		})
		return
	}

	scheduleCfg := scraper.GroupScheduleConfig(group)
	imageFilename, imageData, err := mb.PrepareWeekScheduleData(
		ctx,
		b,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		scheduleCfg,
	)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	err = mb.SendWeekScheduleMessages(ctx, b, update.Message.MessageThreadID, chat, scheduleCfg, imageFilename, imageData)
	addContextHandlerError(ctx, err)
}

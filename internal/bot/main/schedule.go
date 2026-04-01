package mainbot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/botutil"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/service/scraper"
)

func (mb *MainBot) weekHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Week handler")

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(update.Message.Chat.ID).
			Msg("Error in groupHandler")
		return
	}

	if chat.GroupName == nil {
		// Offer to set group
		log.Warn().Any("chat_id", chat.TgChatID).Msg("Group name is not set")
		mb.groupHandler(ctx, b, update)
		return
	}

	group, shouldReturn, err := mb.tryGetGroup(ctx, b, update, chat)
	if shouldReturn {
		addContextHandlerError(ctx, err)
		return
	}

	_, err = b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Action:          tgmodels.ChatActionTyping,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send chat action")
	}

	conf := model.GroupScheduleConfig(group, chat.DarkMode)
	imageFilename, imageData, err := mb.services.Schedule.PrepareScheduleImage(conf)
	if err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	err = mb.SendWeekScheduleMessages(ctx, b, update.Message.MessageThreadID, chat, conf, imageFilename, imageData)
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) SendWeekScheduleMessages(
	ctx context.Context,
	b *bot.Bot,
	messageThreadID int,
	chat *model.Chat,
	conf model.ScheduleConfig,
	imageFilename string,
	imageData []byte,
) error {
	var errs []error

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chat.TgChatID,
		MessageThreadID: messageThreadID,
		Text:            conf.FormatHTML() + ":",
		ParseMode:       tgmodels.ParseModeHTML,
		ReplyMarkup:     botutil.MainMenuReplyMarkup(chat.IsPrivate()),
	}); err != nil {
		errs = append(errs, err)
	}

	if _, err := b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID:          chat.TgChatID,
		MessageThreadID: messageThreadID,
		Action:          tgmodels.ChatActionUploadPhoto,
	}); err != nil {
		errs = append(errs, err)
	}

	replyMarkup := botutil.WeekScheduleMarkup(conf)
	if err := botutil.SendSchedulePhoto(ctx, b, chat, messageThreadID, imageFilename, imageData, replyMarkup); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (mb *MainBot) tomorrowHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Tomorrow handler")

	chatID := update.Message.Chat.ID

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(chatID).Msg("Error in groupHandler")
		return
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Any("chat_id", chat.TgChatID).Msg("Group name is not set")
		mb.groupHandler(ctx, b, update)
		return
	}

	group, shouldReturn, err := mb.tryGetGroup(ctx, b, update, chat)
	if shouldReturn {
		addContextHandlerError(ctx, err)
		return
	}

	rawSchedule, err := mb.services.Schedule.Get(
		model.GroupScheduleConfig(group, chat.DarkMode),
	)
	if err != nil {
		addContextHandlerError(ctx, err)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgCouldNotLoadSchedule,
		})
		return
	}

	schedule := rawSchedule.Transform()
	var tomorrow model.ScheduleDay
	if time.Now().Weekday() == time.Sunday {
		tomorrow = schedule.Days[0]
	} else {
		tomorrow = schedule.Days[1]
	}

	text := tomorrow.HTML()
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
		ParseMode:       tgmodels.ParseModeHTML,
		ReplyMarkup:     updateInlineMarkup("tomorrow", chat.GroupName.String()),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) todayHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Left handler")

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(update.Message.Chat.ID).
			Msg("Error in groupHandler")
		return
	}

	text := ""
	if time.Now().Weekday() == time.Sunday {
		text = "Сегодня воскресенье, отдыхайте!"
	} else {
		if chat.GroupName == nil {
			// Offer to set group
			log.Warn().Any("chat_id", chat.TgChatID).Msg("Group name is not set")
			mb.groupHandler(ctx, b, update)
			return
		}

		group, shouldReturn, err := mb.tryGetGroup(ctx, b, update, chat)
		if shouldReturn {
			addContextHandlerError(ctx, err)
			return
		}

		rawSchedule, err := mb.services.Schedule.Get(
			model.GroupScheduleConfig(group, chat.DarkMode),
		)
		if err != nil {
			addContextHandlerError(ctx, err)
			botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Text:            botutil.ErrMsgCouldNotLoadSchedule,
			})
			return
		}

		schedule := rawSchedule.Transform()
		today := schedule.Today()
		text = today.DynamicFormatHTML(time.Now())
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
		ParseMode:       tgmodels.ParseModeHTML,
		ReplyMarkup:     updateInlineMarkup("left", chat.GroupName.String()),
	})
	addContextHandlerError(ctx, err)
}

func updateInlineMarkup(kind, value string) tgmodels.InlineKeyboardMarkup {
	return tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{{botutil.UpdateInlineButton(kind, value)}},
	}
}

func (mb *MainBot) tryGetGroup(
	ctx context.Context,
	b *bot.Bot,
	update *tgmodels.Update,
	chat *model.Chat,
) (*model.Group, bool, error) {
	group, err := scraper.FetchGroupByNameWithValidation(mb.container.Group, mb.services.Browser, *chat.GroupName)
	if err == nil {
		return group, false, nil
	}

	switch {
	case errors.Is(err, scraper.ErrWrongGroupNameFormat):
		// Should be impossible, since group name is validated before setting it to chat.
		log.Warn().Any("chat_id", chat.TgChatID).Any("group_name", *chat.GroupName).
			Msg("Wrong group name format, offer to set group again")

		chat.DepartmentName = nil
		chat.GroupName = nil
		if err := mb.container.Chat.Update(chat); err != nil {
			botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Text:            botutil.ErrMsgCouldNotUpdateData,
			})
		}

		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgSelectGroupAgain,
		})
		mb.groupHandler(ctx, b, update)
		return nil, true, nil
	case errors.Is(err, scraper.ErrGroupNotFound):
		// Group not found, offer to set group again.

		chat.DepartmentName = nil
		chat.GroupName = nil
		if err := mb.container.Chat.Update(chat); err != nil {
			botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Text:            botutil.ErrMsgCouldNotUpdateData,
			})
		}

		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgSelectGroupAgain,
		})
		mb.groupHandler(ctx, b, update)
		return nil, true, nil
	default:
		botutil.SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            botutil.ErrMsgTryLater,
		})
		return nil, true, fmt.Errorf("failed to fetch group: %w", err)
	}
}

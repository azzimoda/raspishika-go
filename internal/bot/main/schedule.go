package mainbot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/model"
)

func (mb *MainBot) weekHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Week handler")

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
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
	imageFilename, imageData, err := mb.PrepareScheduleImage(conf)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotLoadSchedule,
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
		ReplyMarkup:     mainMenuReplyMarkup(chat.IsPrivate()),
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

	replyMarkup := WeekScheduleMarkup(conf)
	if err := mb.SendSchedulePhoto(ctx, b, chat, messageThreadID, imageFilename, imageData, replyMarkup); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (mb *MainBot) SendSchedulePhoto(
	ctx context.Context,
	b *bot.Bot,
	chat *model.Chat,
	messageThreadID int,
	imageFilename string,
	imageData []byte,
	replyMarkup tgmodels.ReplyMarkup,
) error {
	log.Trace().Any("tgChatID", chat.TgChatID).Str("filename", imageFilename).Msg("Sending schedule photo...")
	_, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:          chat.TgChatID,
		MessageThreadID: messageThreadID,
		Photo:           &tgmodels.InputFileUpload{Filename: imageFilename, Data: bytes.NewReader(imageData)},
		ReplyMarkup:     replyMarkup,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send schedule photo")
		err2 := sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chat.TgChatID,
			Text:   ErrMsgCouldNotSendSchedule,
		})
		return errors.Join(err, err2)
	}
	return err
}

func WeekScheduleMarkup(conf model.ScheduleConfig) tgmodels.ReplyMarkup {
	var button tgmodels.InlineKeyboardButton
	if conf.Group != nil {
		button = updateInlineButton("group", string(conf.Group.GroupName))
	} else if conf.Teacher != nil {
		button = updateInlineButton("teacher", conf.Teacher.TeacherID.String())
	} else {
		return nil
	}
	markup := tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{{button}},
	}
	return markup
}

func updateInlineButton(kind, value string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{
		Text: "Обновить",
		CallbackData: fmt.Sprintf("update_%s\n%s\n%s",
			kind, value,
			time.Now().Format("20060102150405000"), // NOTE: Time is added to prevent editing message error when the content is the same.
		),
	}
}

func scheduleScreenshotFileName(conf model.ScheduleConfig) string {
	darkSuffix := ""
	if conf.IsDark {
		darkSuffix = "_dark"
	}

	if conf.Group != nil {
		return fmt.Sprintf("schedule_%s%s.png", conf.Group.GroupName, darkSuffix)
	} else if conf.Teacher != nil {
		return fmt.Sprintf("schedule_teacher_%s%s.png", conf.Teacher.Name, darkSuffix)
	} else {
		log.Error().Any("config", conf).Msg("Schedule config is invalid")
		return "schedule.png"
	}
}

func (mb *MainBot) tomorrowHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Tomorrow handler")

	chatID := update.Message.Chat.ID

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
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

	rawSchedule, err := mb.services.ScheduleMan.Get(
		mb.services.Browser,
		model.GroupScheduleConfig(group, chat.DarkMode),
	)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          chatID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgCouldNotLoadSchedule,
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
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
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

		rawSchedule, err := mb.services.ScheduleMan.Get(
			mb.services.Browser,
			model.GroupScheduleConfig(group, chat.DarkMode),
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
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{{updateInlineButton(kind, value)}},
	}
}

func (mb *MainBot) tryGetGroup(
	ctx context.Context,
	b *bot.Bot,
	update *tgmodels.Update,
	chat *model.Chat,
) (*model.Group, bool, error) {
	group, err := mb.FetchGroupByNameWithValidation(*chat.GroupName)
	if err == nil {
		return group, false, nil
	}

	switch {
	case errors.Is(err, ErrWrongGroupNameFormat):
		// Should be impossible, since group name is validated before setting it to chat.
		log.Warn().Any("chat_id", chat.TgChatID).Any("group_name", *chat.GroupName).
			Msg("Wrong group name format, offer to set group again")

		chat.DepartmentName = nil
		chat.GroupName = nil
		if err := mb.services.Container.Chat.Update(chat); err != nil {
			sendErrorMessage(ctx, b, &bot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Text:            ErrMsgCouldNotUpdateData,
			})
		}

		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgSelectGroupAgain,
		})
		mb.groupHandler(ctx, b, update)
		return nil, true, nil
	case errors.Is(err, ErrGroupNotFound):
		// Group not found, offer to set group again.

		chat.DepartmentName = nil
		chat.GroupName = nil
		if err := mb.services.Container.Chat.Update(chat); err != nil {
			sendErrorMessage(ctx, b, &bot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Text:            ErrMsgCouldNotUpdateData,
			})
		}

		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgSelectGroupAgain,
		})
		mb.groupHandler(ctx, b, update)
		return nil, true, nil
	default:
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            ErrMsgTryLater,
		})
		return nil, true, fmt.Errorf("failed to fetch group: %w", err)
	}
}

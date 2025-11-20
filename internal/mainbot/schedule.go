package mainbot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
)

func (mb *MainBot) weekHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Week handler")

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgTryLater})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(update.Message.Chat.ID).
			Msg("Error in groupHandler")
		return
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Int64("chat_id", chat.TgChatID).Msg("Group name is not set")
		mb.groupHandler(ctx, b, update)
		return
	}

	group, shouldReturn, err := mb.tryGetGroup(ctx, b, update, chat)
	if shouldReturn {
		addContextHandlerError(ctx, err)
		return
	}

	scheduleCfg := scraper.GroupScheduleConfig(group)
	mb.sendWeekSchedule(ctx, b, &update.Message.Chat, scheduleCfg)
}

func (mb *MainBot) sendWeekSchedule(
	ctx context.Context,
	b *bot.Bot,
	chat *models.Chat,
	scheduleCfg scraper.ScheduleConfig,
) {
	log.Trace().Msg("Sending week schedule")

	chatID := chat.ID
	_, err := b.SendChatAction(ctx, &bot.SendChatActionParams{ChatID: chatID, Action: models.ChatActionTyping})
	addContextHandlerError(ctx, err)

	schedule, err := mb.services.ScheduleManager.Get(
		mb.services.Repo, mb.services.Browser, mb.services.Cache, scheduleCfg)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgCouldNotLoadSchedule})
		return
	}

	html := schedule.HTML(mb.config.ScheduleTemplate)
	imageFilename := path.Join(mb.config.Browser.ScreenshotDir, scheduleScreenshotFileName(scheduleCfg))
	if err := mb.services.Browser.TakeScreenshotHTML(html, imageFilename); err != nil {
		// TODO: Try to send old screenshot.
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgCouldNotLoadSchedule})
		return
	}

	imageData, err := os.ReadFile(imageFilename)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgCouldNotLoadSchedule})
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        scheduleCfg.FormatMarkdown() + ":",
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: mainMenuReplyMarkup(chat.Type == models.ChatTypePrivate),
	})
	addContextHandlerError(ctx, err)

	_, err = b.SendChatAction(ctx, &bot.SendChatActionParams{ChatID: chatID, Action: models.ChatActionUploadPhoto})
	addContextHandlerError(ctx, err)

	_, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:      chatID,
		Photo:       &models.InputFileUpload{Filename: imageFilename, Data: bytes.NewReader(imageData)},
		ReplyMarkup: weekScheduleMarkup(scheduleCfg),
	})
	addContextHandlerError(ctx, err)
}

func weekScheduleMarkup(config scraper.ScheduleConfig) models.ReplyMarkup {
	var button models.InlineKeyboardButton
	if config.Group != nil {
		button = updateInlineButton("group", config.Group.GroupName)
	} else if config.Teacher != nil {
		button = updateInlineButton("teacher", config.Teacher.TeacherID)
	} else {
		return nil
	}
	markup := models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{button}},
	}
	log.Trace().Any("config", config).Any("markup", markup).Msg("Week schedule markup")
	return markup
}

func updateInlineButton(kind, value string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{
		Text:         "Обновить",
		CallbackData: fmt.Sprintf("update_%s\n%s", kind, value),
	}
}

func scheduleScreenshotFileName(config scraper.ScheduleConfig) string {
	if config.Group != nil {
		return fmt.Sprintf("schedule_%s.png", config.Group.GroupName)
	} else if config.Teacher != nil {
		return fmt.Sprintf("schedule_%s.png", config.Teacher.Name)
	} else {
		log.Error().Any("config", config).Msg("Schedule config is invalid")
		return "schedule.png"
	}
}

func (mb *MainBot) tomorrowHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Tomorrow handler")

	chatID := update.Message.Chat.ID

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgTryLater})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(chatID).Msg("Error in groupHandler")
		return
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Int64("chat_id", chat.TgChatID).Msg("Group name is not set")
		mb.groupHandler(ctx, b, update)
		return
	}

	group, shouldReturn, err := mb.tryGetGroup(ctx, b, update, chat)
	if shouldReturn {
		addContextHandlerError(ctx, err)
		return
	}

	rawSchedule, err := mb.services.ScheduleManager.Get(
		mb.services.Repo, mb.services.Browser, mb.services.Cache, scraper.GroupScheduleConfig(group))
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgCouldNotLoadSchedule})
		return
	}

	schedule := rawSchedule.Transform()
	var tomorrow scraper.ScheduleDay
	if time.Now().Weekday() == time.Sunday {
		tomorrow = schedule.Days[0]
	} else {
		tomorrow = schedule.Days[1]
	}

	text := tomorrow.String()
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: updateInlineMarkup("tomorrow", *chat.GroupName),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) leftHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Left handler")

	chatID := update.Message.Chat.ID

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgTryLater})
		mb.services.Reporter.Report().Log().Err(ErrNoChatContext).Chat(chatID).Msg("Error in groupHandler")
		return
	}

	if chat.GroupName == nil {
		// Offer to set group.
		log.Warn().Int64("chat_id", chat.TgChatID).Msg("Group name is not set")
		mb.groupHandler(ctx, b, update)
		return
	}

	group, shouldReturn, err := mb.tryGetGroup(ctx, b, update, chat)
	if shouldReturn {
		addContextHandlerError(ctx, err)
		return
	}

	rawSchedule, err := mb.services.ScheduleManager.Get(
		mb.services.Repo, mb.services.Browser, mb.services.Cache, scraper.GroupScheduleConfig(group))
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: chatID, Text: ErrMsgCouldNotLoadSchedule})
		return
	}

	schedule := rawSchedule.Transform()
	left := schedule.Days[0].Left()
	text := ""
	if left.IsEmpty() {
		text = "Сегодня больше нет пар"
	} else {
		text = left.String()
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: updateInlineMarkup("left", *chat.GroupName),
	})
	addContextHandlerError(ctx, err)
}

func updateInlineMarkup(kind, value string) models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{updateInlineButton(kind, value)}},
	}
}

func (mb *MainBot) tryGetGroup(
	ctx context.Context,
	b *bot.Bot,
	update *models.Update,
	chat *database.Chat,
) (*database.Group, bool, error) {
	group, err := mb.fetchGroupByNameWithValidation(*chat.GroupName)
	if err == nil {
		return group, false, nil
	}

	switch {
	case errors.Is(err, ErrWrongGroupNameFormat):
		// Should be impossible, since group name is validated before setting it to chat.
		log.Warn().Int64("chat_id", chat.TgChatID).Str("group_name", *chat.GroupName).
			Msg("Wrong group name format, offer to set group again")

		chat.DepartmentName = nil
		chat.GroupName = nil
		if err := mb.services.Repo.UpdateChat(chat); err != nil {
			sendErrorMessage(ctx, b, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   ErrMsgCouldNotUpdateData,
			})
		}

		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgSelectGroupAgain})
		mb.groupHandler(ctx, b, update)
		return nil, true, nil
	case errors.Is(err, ErrGroupNotFound):
		// Group not found, offer to set group again.

		chat.DepartmentName = nil
		chat.GroupName = nil
		if err := mb.services.Repo.UpdateChat(chat); err != nil {
			sendErrorMessage(ctx, b, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   ErrMsgCouldNotUpdateData,
			})
		}

		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   ErrMsgSelectGroupAgain,
		})
		mb.groupHandler(ctx, b, update)
		return nil, true, nil
	default:
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: ErrMsgTryLater})
		return nil, true, fmt.Errorf("failed to fetch group: %w", err)
	}
}

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
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/ninetwentyfour/go-wkhtmltoimage"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
)

func (mb *MainBot) weekHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Week handler")

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)

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

	_, err = b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Action:          tgmodels.ChatActionTyping,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send chat action")
	}

	conf := models.GroupScheduleConfig(group)
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
	log.Debug().Str("filename", imageFilename).Msg("Screenshot saved")

	err = mb.SendWeekScheduleMessages(ctx, b, update.Message.MessageThreadID, chat, conf, imageFilename, imageData)
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) PrepareScheduleImage(conf models.ScheduleConfig) (
	imageFilename string,
	imageData []byte,
	err error,
) {
	schedule, err := mb.services.ScheduleManager.Get(mb.services.Repo, mb.services.Browser, conf)
	if err != nil {
		err = fmt.Errorf("failed loading schedule: %w", err)
		return
	}

	html := schedule.HTML(viper.GetString("schedule_template"))

	imageFilename, imageData, err = mb.htmlToImage(conf, html)
	if err != nil {
		return "", nil, err
	}
	return imageFilename, imageData, nil
}

func (mb *MainBot) htmlToImage(
	scheduleCfg models.ScheduleConfig,
	html string,
) (string, []byte, error) {
	imageFilename := path.Join(
		viper.GetString("browser.screenshot_dir"),
		scheduleScreenshotFileName(scheduleCfg),
	)
	if err := mb.services.Browser.TakeScreenshotHTML(html, imageFilename); err != nil {
		return "", nil, err
	}

	imageData, err := os.ReadFile(imageFilename)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read screenshot: %w", err)
	}
	return imageFilename, imageData, nil
}

func htmlToImage(scheduleCfg models.ScheduleConfig, html, imageFilename string) (imageDage []byte, err error) {
	htmlFilename, err := scraper.SaveScheduleHTML(scheduleCfg, html)
	if err != nil {
		return nil, err
	}

	return wkhtmltoimage.GenerateImage(&wkhtmltoimage.ImageOptions{
		Input:  htmlFilename,
		Format: "png",
		Output: imageFilename,
	})
}

func (mb *MainBot) SendWeekScheduleMessages(
	ctx context.Context,
	b *bot.Bot,
	messageThreadID int,
	chat *models.Chat,
	conf models.ScheduleConfig,
	imageFilename string,
	imageData []byte,
) error {
	var errs []error

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chat.TgChatID,
		MessageThreadID: messageThreadID,
		Text:            conf.FormatMarkdown() + ":",
		ParseMode:       tgmodels.ParseModeMarkdown,
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
	chat *models.Chat,
	messageThreadID int,
	imageFilename string,
	imageData []byte,
	replyMarkup tgmodels.ReplyMarkup,
) error {
	_, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:          chat.TgChatID,
		MessageThreadID: messageThreadID,
		Photo:           &tgmodels.InputFileUpload{Filename: imageFilename, Data: bytes.NewReader(imageData)},
		ReplyMarkup:     replyMarkup,
	})
	return err
}

func WeekScheduleMarkup(config models.ScheduleConfig) tgmodels.ReplyMarkup {
	var button tgmodels.InlineKeyboardButton
	if config.Group != nil {
		button = updateInlineButton("group", config.Group.GroupName)
	} else if config.Teacher != nil {
		button = updateInlineButton("teacher", config.Teacher.TeacherID)
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
		Text:         "Обновить",
		CallbackData: fmt.Sprintf("update_%s\n%s", kind, value),
	}
}

func scheduleScreenshotFileName(config models.ScheduleConfig) string {
	if config.Group != nil {
		return fmt.Sprintf("schedule_%s.png", config.Group.GroupName)
	} else if config.Teacher != nil {
		return fmt.Sprintf("schedule_%s.png", config.Teacher.Name)
	} else {
		log.Error().Any("config", config).Msg("Schedule config is invalid")
		return "schedule.png"
	}
}

func (mb *MainBot) tomorrowHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Tomorrow handler")

	chatID := update.Message.Chat.ID

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
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
		mb.services.Repo, mb.services.Browser, models.GroupScheduleConfig(group))
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
	var tomorrow models.ScheduleDay
	if time.Now().Weekday() == time.Sunday {
		tomorrow = schedule.Days[0]
	} else {
		tomorrow = schedule.Days[1]
	}

	text := tomorrow.String()
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chatID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
		ParseMode:       tgmodels.ParseModeMarkdown,
		ReplyMarkup:     updateInlineMarkup("tomorrow", *chat.GroupName),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) leftHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Left handler")

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
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
		text = `Сегодня воскресенье, отдыхайте\!`
	} else {
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
			mb.services.Repo,
			mb.services.Browser,
			models.GroupScheduleConfig(group),
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
		left := schedule.Days[0].Left()

		if left.IsEmpty() {
			text = "Сегодня больше нет пар"
		} else {
			text = left.String()
		}
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            text,
		ParseMode:       tgmodels.ParseModeMarkdown,
		ReplyMarkup:     updateInlineMarkup("left", *chat.GroupName),
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
	chat *models.Chat,
) (*models.Group, bool, error) {
	group, err := mb.FetchGroupByNameWithValidation(*chat.GroupName)
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
		if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
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
		if err := models.UpdateChat(mb.services.Repo.DB, chat); err != nil {
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

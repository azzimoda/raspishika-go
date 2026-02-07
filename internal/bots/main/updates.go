package mainbot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

func (mb *MainBot) updateGroupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Update group handler")

	message := update.CallbackQuery.Message.Message
	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	groupName := command.Arg(0)

	group, err := mb.services.Repo.GetGroupByName(groupName)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	scheduleCfg := scraper.GroupScheduleConfig(group)
	schedule, err := mb.services.ScheduleManager.Get(
		mb.services.Repo,
		mb.services.Browser,
		mb.services.Cache,
		scheduleCfg,
	)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	html := schedule.HTML(viper.GetString("schedule_template"))
	imageFilename := filepath.Join(viper.GetString("browser.screenshot_dir"), scheduleScreenshotFileName(scheduleCfg))
	if err := mb.services.Browser.TakeScreenshotHTML(html, imageFilename); err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		addContextHandlerError(ctx, err)
		return
	}

	imageData, err := os.ReadFile(imageFilename)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgCouldNotLoadSchedule})
		return
	}

	_, err = b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
		ChatID:      message.Chat.ID,
		MessageID:   message.ID,
		Media:       &models.InputMediaPhoto{Media: "attach://image.png", MediaAttachment: bytes.NewReader(imageData)},
		ReplyMarkup: updateInlineMarkup("group", groupName),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) updateTeacherHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Update teacher handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	teacherID := command.Arg(0)

	teacher, err := mb.services.Repo.GetTeacherByTeacherID(teacherID)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	scheduleCfg := scraper.TeacherScheduleConfig(teacher)
	schedule, err := mb.services.ScheduleManager.Get(
		mb.services.Repo,
		mb.services.Browser,
		mb.services.Cache,
		scheduleCfg,
	)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	html := schedule.HTML(viper.GetString("schedule_template"))
	imageFilename := filepath.Join(viper.GetString("browser.screenshot_dir"), scheduleScreenshotFileName(scheduleCfg))
	if err := mb.services.Browser.TakeScreenshotHTML(html, imageFilename); err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		addContextHandlerError(ctx, err)
		return
	}

	message := update.CallbackQuery.Message.Message

	imageData, err := os.ReadFile(imageFilename)
	if err != nil {
		addContextHandlerError(ctx, err)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{ChatID: message.Chat.ID, Text: ErrMsgCouldNotLoadSchedule})
		return
	}

	_, err = b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
		ChatID:      message.Chat.ID,
		MessageID:   message.ID,
		Media:       &models.InputMediaPhoto{Media: "attach://image.png", MediaAttachment: bytes.NewReader(imageData)},
		ReplyMarkup: updateInlineMarkup("teacher", teacherID),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) updateTomorrowHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Update tomorrow handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	groupName := command.Arg(0)

	group, err := mb.services.Repo.GetGroupByName(groupName)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	scheduleCfg := scraper.GroupScheduleConfig(group)
	rawSchedule, err := mb.services.ScheduleManager.Get(
		mb.services.Repo,
		mb.services.Browser,
		mb.services.Cache,
		scheduleCfg,
	)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	schedule := rawSchedule.Transform()
	var tomorrow scraper.ScheduleDay
	if time.Now().Weekday() == time.Sunday {
		tomorrow = schedule.Days[0]
	} else {
		tomorrow = schedule.Days[1]
	}

	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        tomorrow.String(),
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: updateInlineMarkup("tomorrow", groupName),
	})
	if errors.Is(err, bot.ErrorBadRequest) && strings.Contains(err.Error(), "message is not modified") {
		log.Debug().Msg("Message is not modified")
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Ничего не изменилось",
		})
		addContextHandlerError(ctx, err)
		return
	}
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) updateLeftHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Update left handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	groupName := command.Arg(0)

	group, err := mb.services.Repo.GetGroupByName(groupName)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	text := ""
	if time.Now().Weekday() == time.Sunday {
		text = `Сегодня воскресенье, отдыхайте\!`
	} else {
		scheduleCfg := scraper.GroupScheduleConfig(group)
		rawSchedule, err := mb.services.ScheduleManager.Get(
			mb.services.Repo,
			mb.services.Browser,
			mb.services.Cache,
			scheduleCfg,
		)
		if err != nil {
			addContextHandlerError(ctx, err)
			_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            ErrMsgCouldNotLoadSchedule,
			})
			addContextHandlerError(ctx, err)
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

	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        text,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: updateInlineMarkup("left", groupName),
	})
	if errors.Is(err, bot.ErrorBadRequest) && strings.Contains(err.Error(), "message is not modified") {
		log.Debug().Msg("Message is not modified")
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Ничего не изменилось",
		})
		addContextHandlerError(ctx, err)
		return
	}
	addContextHandlerError(ctx, err)
}

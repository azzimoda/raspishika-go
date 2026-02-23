package mainbot

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

func (mb *MainBot) updateGroupHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Update group handler")

	message := update.CallbackQuery.Message.Message
	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	groupName := command.Arg(0)

	group, err := models.GetGroupByName(mb.services.Repo.DB, groupName)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	conf := models.GroupScheduleConfig(group)
	_, imageData, err := mb.PrepareScheduleImage(conf)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	_, err = b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
		ChatID:      message.Chat.ID,
		MessageID:   message.ID,
		Media:       &tgmodels.InputMediaPhoto{Media: "attach://image.png", MediaAttachment: bytes.NewReader(imageData)},
		ReplyMarkup: updateInlineMarkup("group", groupName),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) updateTeacherHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Update teacher handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	teacherID := command.Arg(0)

	teacher, err := models.GetTeacherByTeacherID(mb.services.Repo.DB, teacherID)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	conf := models.TeacherScheduleConfig(teacher)
	_, imageData, err := mb.PrepareScheduleImage(conf)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotUpdateData,
		})
		addContextHandlerError(ctx, err)
		return
	}

	_, err = b.EditMessageMedia(ctx, &bot.EditMessageMediaParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Media:       &tgmodels.InputMediaPhoto{Media: "attach://image.png", MediaAttachment: bytes.NewReader(imageData)},
		ReplyMarkup: updateInlineMarkup("teacher", teacherID),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) updateTomorrowHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Update tomorrow handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	groupName := command.Arg(0)

	group, err := models.GetGroupByName(mb.services.Repo.DB, groupName)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	rawSchedule, err :=
		mb.services.ScheduleMan.Get(mb.services.Repo, mb.services.Browser, models.GroupScheduleConfig(group))
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
	tomorrow := schedule.Tomorrow(time.Now())

	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        tomorrow.String(),
		ParseMode:   tgmodels.ParseModeMarkdown,
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

func (mb *MainBot) updateLeftHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Update left handler")

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	groupName := command.Arg(0)

	group, err := models.GetGroupByName(mb.services.Repo.DB, groupName)
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
		conf := models.GroupScheduleConfig(group)
		rawSchedule, err := mb.services.ScheduleMan.Get(mb.services.Repo, mb.services.Browser, conf)
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
		ParseMode:   tgmodels.ParseModeMarkdown,
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

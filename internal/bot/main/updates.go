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

	"github.com/azzimoda/raspishika-go/internal/model"
	bothelpers "github.com/azzimoda/raspishika-go/pkg/bothelper"
)

func (mb *MainBot) updateGroupHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Update group handler")

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	darkMode := false
	if !ok {
		addContextHandlerError(ctx, errors.New("chat not found in context"))
		log.Error().Err(errors.New("chat not found in context")).Send()
	} else {
		darkMode = chat.DarkMode
	}

	message := update.CallbackQuery.Message.Message
	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	groupName := model.GroupName(command.Arg(0))

	group, err := mb.services.Group.GetByName(groupName)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	conf := model.GroupScheduleConfig(group, darkMode)
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
		ReplyMarkup: updateInlineMarkup("group", groupName.String()),
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) updateTeacherHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Update teacher handler")

	chat, ok := ctx.Value(chatContextKey).(*model.Chat)
	darkMode := false
	if !ok {
		addContextHandlerError(ctx, errors.New("chat not found in context"))
		log.Error().Err(errors.New("chat not found in context")).Send()
		return
	} else {
		darkMode = chat.DarkMode
	}

	command := bothelpers.ParseCallbackData(update.CallbackQuery.Data)
	teacherID := command.Arg(0)

	teacher, err := mb.services.Group.GetTeacherByID(teacherID)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	conf := model.TeacherScheduleConfig(teacher, darkMode)
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
	groupName := model.GroupName(command.Arg(0))

	group, err := mb.services.Group.GetByName(groupName)
	if err != nil {
		addContextHandlerError(ctx, err)
		_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            ErrMsgCouldNotLoadSchedule,
		})
		addContextHandlerError(ctx, err)
		return
	}

	rawSchedule, err := mb.services.ScheduleMan.Get(mb.services.Browser, model.GroupScheduleConfig(group, false))
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
		Text:        tomorrow.HTML(),
		ParseMode:   tgmodels.ParseModeHTML,
		ReplyMarkup: updateInlineMarkup("tomorrow", groupName.String()),
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
	groupName := model.GroupName(command.Arg(0))

	group, err := mb.services.Group.GetByName(groupName)
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
		conf := model.GroupScheduleConfig(group, false)
		rawSchedule, err := mb.services.ScheduleMan.Get(mb.services.Browser, conf)
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
		today := schedule.Today()
		text = today.DynamicFormatHTML(time.Now())
	}

	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      update.CallbackQuery.Message.Message.Chat.ID,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		Text:        text,
		ParseMode:   tgmodels.ParseModeHTML,
		ReplyMarkup: updateInlineMarkup("left", groupName.String()),
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

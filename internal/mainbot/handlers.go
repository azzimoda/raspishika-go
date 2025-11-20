package mainbot

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
)

var (
	ErrNoChatContext error = errors.New("failed to get chat from context")
)

const StartMessage = `Привет! Я предостовляю удобный способ получения расписания МПК ТИУ.

Для начала нужно задать свою группу для использования комманд /week, /tomorrow, /left и получения рассылки. Други комманды и функции перечислены в /help.

Помимо команд можно использовать кнопки клавиатуры, а также меня можно добавить в групповой чат.

Подпишись на канал разработчика @mazzaLLM, где ты можешь найти новости о боте и поделиться своим мнением в комментариях.`

const HelpMessage = `Доступные команды:

• /week — Расписание на неделю
• /tomorrow — Расписание на завтра
• /left — Оставшиеся пары
• /quick — Расписание другой группы
• /teacher — Расписание преподавателя

• /settings — Меню настроек
• /group — Изменить свою группу
• /daily_time — Настроить ежедневную рассылку
• /daily_off — Выключить ежедневную рассылку
• /reminder_on — Включить напоминания перед парами
• /reminder_off — Выключить напоминания перед парами
• /access — Изменить уровень доступа к командам бота в групповом чате

• /stop — Удалить данные о себе и остановить рассылки
• /help — Это сообщение

Прочие функции:

• Бота можно добавить в групповой чат
• Напоминание приходит в течение 15 минут до начала пары

По всем вопросам обращайтесь к расработчику @MazzzaRellla или пишите в комментарии канала @mazzaLLM.`

func (mb *MainBot) registerHandlers() {
	// Commands
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommandStartOnly,
		mb.startHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "help", bot.MatchTypeCommandStartOnly,
		mb.helpHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "stop", bot.MatchTypeCommandStartOnly,
		mb.stopHandler, mb.checkRegularAccessMiddleware)

	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "settings", bot.MatchTypeCommandStartOnly,
		mb.settingsHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "group", bot.MatchTypeCommandStartOnly,
		mb.groupHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "daily_time", bot.MatchTypeCommandStartOnly,
		mb.dailyTimeHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "daily_off", bot.MatchTypeCommandStartOnly,
		mb.dailyOffHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "reminder_on", bot.MatchTypeCommandStartOnly,
		mb.reminderOnHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "reminder_off", bot.MatchTypeCommandStartOnly,
		mb.reminderOffHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "access", bot.MatchTypeCommandStartOnly,
		mb.accessHandler, mb.checkConfigAccessMiddleware)

	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "week", bot.MatchTypeCommandStartOnly,
		mb.weekHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "tomorrow", bot.MatchTypeCommandStartOnly,
		mb.tomorrowHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "left", bot.MatchTypeCommandStartOnly,
		mb.leftHandler, mb.checkRegularAccessMiddleware)

	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "quick", bot.MatchTypeCommandStartOnly,
		mb.quickHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandler(bot.HandlerTypeMessageText, "teacher", bot.MatchTypeCommandStartOnly,

		mb.teacherHandler, mb.checkRegularAccessMiddleware)

	// Text messages
	// TODO: Check whether case-insensitive matching works for Cyrrilic letters.
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile("(?i)^неделя$"),
		mb.weekHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile("(?i)^завтра$"),
		mb.tomorrowHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile("(?i)^сегодня$"),
		mb.leftHandler, mb.checkRegularAccessMiddleware)

	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile("(?i)^другая группа$"),
		mb.quickHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile("(?i)^преподаватель$"),
		mb.teacherHandler, mb.checkRegularAccessMiddleware)

	mb.Bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.Message != nil && update.Message.Text == "отмена"
	}, mb.textCancelHandler, mb.checkRegularAccessMiddleware)

	mb.Bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		if update.Message == nil {
			return false
		}

		chat, err := mb.services.Repo.GetChatByTgChatID(update.Message.Chat.ID)
		if err != nil || chat == nil {
			log.Error().Err(err).Int64("tgChatID", update.Message.Chat.ID).Msg("failed to get chat by chat ID")
			return false
		}
		return chat.State == database.ChatStateSelectingGroup && update.Message.Text != "отмена"
	}, mb.textGroupHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		if update.Message == nil {
			return false
		}

		chat, err := mb.services.Repo.GetChatByTgChatID(update.Message.Chat.ID)
		if err != nil || chat == nil {
			log.Error().Err(err).Int64("tgChatID", update.Message.Chat.ID).Msg("failed to get chat by chat ID")
			return false
		}
		return chat.State == database.ChatStateQuickSelectingGroup && update.Message.Text != "отмена"
	}, mb.textQuickGroupHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		if update.Message == nil {
			return false
		}

		chat, err := mb.services.Repo.GetChatByTgChatID(update.Message.Chat.ID)
		if err != nil || chat == nil {
			log.Error().Err(err).Int64("tgChatID", update.Message.Chat.ID).Msg("failed to get chat by chat ID")
			return false
		}
		return chat.State == database.ChatStateSelectingTime && update.Message.Text != "отмена"
	}, mb.textTimeHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		if update.Message == nil {
			return false
		}

		chat, err := mb.services.Repo.GetChatByTgChatID(update.Message.Chat.ID)
		if err != nil || chat == nil {
			log.Error().Err(err).Int64("tgChatID", update.Message.Chat.ID).Msg("failed to get chat by chat ID")
			return false
		}
		return chat.State == database.ChatStateSelectingTeacher && update.Message.Text != "отмена"
	}, mb.textTeacherNameHandler, mb.checkRegularAccessMiddleware)

	// Callback queries
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("delete"),
		mb.deleteHandler, mb.checkRegularAccessMiddleware)
	// mb.bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("delete_config"),
	// 	mb.deleteHandler, mb.checkConfigAccessMiddleware)

	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("config_group"),
		mb.configGroupHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("config_daily_time"),
		mb.configDailyTimeHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("daily_off"),
		mb.dailyOffCallbackHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("config_reminder"),
		mb.configReminderHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("config_access"),
		mb.configAccessHandler, mb.checkConfigAccessMiddleware)

	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("select_department"),
		mb.selectDepartmentHandler, mb.checkConfigAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("quick_select_department"),
		mb.quickSelectDepartmentHandler, mb.checkRegularAccessMiddleware)

	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("select_teacher"),
		mb.selectTeacherHandler, mb.checkRegularAccessMiddleware)

	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("set_access"),
		mb.setAccessHandler, mb.checkConfigAccessMiddleware)

	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("update_group"),
		mb.updateGroupHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("update_teacher"),
		mb.updateTeacherHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("update_tomorrow"),
		mb.updateTomorrowHandler, mb.checkRegularAccessMiddleware)
	mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("update_left"),
		mb.updateLeftHandler, mb.checkRegularAccessMiddleware)
}

func callbackDataRegexp(command string) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf("^%s(\n.*)*$", command))
}

// Basic handlers

func (mb *MainBot) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Default handler")

	defaultHandlerFlag := ctx.Value(defaultHandlerContextKey).(*bool)
	*defaultHandlerFlag = true

	if update.CallbackQuery != nil {
		_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Это сообщение больше не поддерживается",
		})
		addContextHandlerError(ctx, err)
	}
}

func (mb *MainBot) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Start handler")
	_, err := mb.Bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: StartMessage})
	addContextHandlerError(ctx, err)

	if err == nil {
		chat, err := mb.services.Repo.GetChatByTgChatID(update.Message.Chat.ID)
		if err == nil && chat.GroupName == nil {
			mb.groupHandler(ctx, b, update)
		} else {
			addContextHandlerError(ctx, fmt.Errorf("failed to get chat by chat ID: %w", err))
		}
	}
}

func (mb *MainBot) helpHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Help handler")
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: HelpMessage})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) stopHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Stop handler")

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось удалить данные чата, попробуйте позже",
		})
		return
	}

	if err := mb.services.Repo.DeleteChat(chat.ID); err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to delete chat: %w", err))
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Не удалось удалить данные чата, попробуйте позже",
		})
		return
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "Рассылки остановлены и ваши данные удалены. Спасибо, что пользовались нашим ботом!",
		ReplyMarkup: models.ReplyKeyboardRemove{RemoveKeyboard: true},
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) deleteHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Delete handler")

	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
	})
	addContextHandlerError(ctx, err)

	if repoErr := mb.services.Repo.UpdateChatState(
		update.CallbackQuery.Message.Message.Chat.ID,
		database.ChatStateDefault,
	); repoErr != nil {
		mb.services.Reporter.Report().Log().Err(repoErr).Chat(update.CallbackQuery.Message.Message.Chat.ID).
			Msg("Error in deleteHandler")
		addContextHandlerError(ctx, fmt.Errorf("failed to update chat state: %w", repoErr))
		return
	}
}

func (mb *MainBot) textCancelHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Text cancel handler")

	_, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: update.Message.Chat.ID, MessageID: update.Message.ID})
	addContextHandlerError(ctx, err)

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		mb.services.Reporter.Report().Log().Err(fmt.Errorf("failed to get chat from context")).
			Chat(update.Message.Chat.ID).Msg("Error in textCancelHandler")
		return
	}

	chat.State = database.ChatStateDefault
	if err := mb.services.Repo.UpdateChat(chat); err != nil {
		mb.services.Reporter.Report().Log().Err(err).Chat(update.Message.Chat.ID).Msg("Error in textCancelHandler")
		addContextHandlerError(ctx, fmt.Errorf("failed to update chat: %w", err))
		return
	}

	err = tgbothelpers.SendTempMessage(ctx, b, 3*time.Second, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        "Действие отменено",
		ReplyMarkup: mainMenuReplyMarkup(update.Message.Chat.ID > 0),
	})
	addContextHandlerError(ctx, err)
}

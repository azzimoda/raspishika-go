package mainbot

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	database "github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
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

• Чтобы получить расписание любой группы, пришлите её название, например: "ИСПт-22-(9)-2", или "испт 22 9 2"
• Бота можно добавить в групповой чат
• Напоминание приходит в течение 15 минут до начала пары

По всем вопросам пишите в комментарии или в директ канала @mazzaLLM.`

func (mb *MainBot) registerHandlers() {
	// Commands
	{
		mb.registerCommandHandler("start", mb.startHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("help", mb.helpHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("stop", mb.stopHandler, mb.checkRegularAccessMiddleware)

		mb.registerCommandHandler("settings", mb.settingsHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("group", mb.groupHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("daily_time", mb.dailyTimeHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("daily_off", mb.dailyOffHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("reminder_on", mb.reminderOnHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("reminder_off", mb.reminderOffHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("access", mb.accessHandler, mb.checkConfigAccessMiddleware)

		mb.registerCommandHandler("week", mb.weekHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("tomorrow", mb.tomorrowHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("left", mb.leftHandler, mb.checkRegularAccessMiddleware)

		mb.registerCommandHandler("teacher", mb.teacherHandler, mb.checkRegularAccessMiddleware)
	}

	// Text messages
	{
		mb.registerTextMessageHandler("неделя", mb.weekHandler, mb.checkRegularAccessMiddleware)
		mb.registerTextMessageHandler("завтра", mb.tomorrowHandler, mb.checkRegularAccessMiddleware)
		mb.registerTextMessageHandler("сегодня", mb.leftHandler, mb.checkRegularAccessMiddleware)

		mb.registerTextMessageHandler("преподаватель", mb.teacherHandler, mb.checkRegularAccessMiddleware)

		mb.Bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			return update.Message != nil && strings.ToLower(update.Message.Text) == "отмена"
		}, mb.textCancelHandler, mb.checkRegularAccessMiddleware)

		mb.registerChatStateHandler(database.ChatStateSelectingGroup, mb.textGroupHandler,
			mb.checkConfigAccessMiddleware)
		mb.registerChatStateHandler(database.ChatStateSelectingTime, mb.textTimeHandler,
			mb.checkConfigAccessMiddleware)
		mb.registerChatStateHandler(database.ChatStateSelectingTeacher, mb.textTeacherNameHandler,
			mb.checkRegularAccessMiddleware)
	}

	// Callback queries
	{
		mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("delete"),
			mb.deleteHandler, mb.checkRegularAccessMiddleware)
		mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("delete_config"),
			mb.deleteHandler, mb.checkConfigAccessMiddleware)

		mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("config_group"),
			mb.configGroupHandler, mb.checkConfigAccessMiddleware)
		mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("config_daily_time"),
			mb.configDailyTimeHandler, mb.checkConfigAccessMiddleware)
		mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("daily_off"),
			mb.dailyOffCallbackHandler, mb.checkConfigAccessMiddleware)
		mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("config_reminder"),
			mb.configReminderHandler, mb.checkConfigAccessMiddleware)
		mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("config_set_access"),
			mb.configAccessHandler, mb.checkConfigAccessMiddleware)

		mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp("select_department"),
			mb.selectDepartmentHandler, mb.checkConfigAccessMiddleware)

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
}

func (mb *MainBot) registerCommandHandler(pattern string, f bot.HandlerFunc, m ...bot.Middleware) string {
	matchFunc := commandMatchFunction(pattern, mb.Me.Username)
	return mb.Bot.RegisterHandlerMatchFunc(matchFunc, f, m...)
}

func commandMatchFunction(pattern string, username string) bot.MatchFunc {
	re := regexp.MustCompile(fmt.Sprintf(`^/%s(@\w+)?(\s[\s\S]+)?$`, pattern))
	return func(update *models.Update) bool {
		return matchUpdatePatternUsername(update, pattern, username, re)
	}
}

func matchUpdatePatternUsername(update *models.Update, pattern string, username string, re *regexp.Regexp) bool {
	if update.Message == nil || update.Message.Text == "" {
		return false // Not a text message.
	}

	text := update.Message.Text
	log.Trace().Str("pattern", pattern).Str("username", username).Str("text", text).Msg("Checking command match...")

	if !re.MatchString(text) {
		log.Trace().Str("text", text).Msg("Invalid command")
		return false
	}
	submatches := re.FindStringSubmatch(text)
	if submatches == nil {
		log.Trace().Str("text", text).Msg("No submatches")
		return true
	}
	log.Trace().Str("text", text).Strs("submatches", submatches).Send()

	if update.Message.Chat.Type == models.ChatTypeGroup || update.Message.Chat.Type == models.ChatTypeSupergroup {
		log.Trace().Str("text", text).Msg("Command in group chat")

		if submatches[1] == "" {
			// In group chats command without username are sent to last accessed bot.
			// That means, if this bot is not the last accessed, it won't receive that message.
			// Otherwise, it will receive it and should handle it.
			log.Trace().Str("text", text).Msg("Command without username")
			return true
		}

		if submatches[1] == "@"+username {
			log.Trace().Str("text", text).Msg("Command with my username")
			return true
		}

		log.Trace().Str("text", text).Msg("Not my username")
		return false
	}
	log.Trace().Str("text", text).Msg("Command in private chat")

	// In private chat any username is allowed.
	return true
}

func (mb *MainBot) registerTextMessageHandler(pattern string, f bot.HandlerFunc, m ...bot.Middleware) string {
	return mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(fmt.Sprintf("(?i)^%s$", pattern)), f, m...)
}

func (mb *MainBot) registerChatStateHandler(
	chatState database.ChatState,
	f bot.HandlerFunc,
	m ...bot.Middleware,
) string {
	matchFunc := func(update *models.Update) bool {
		if update.Message == nil {
			return false
		}

		chat, err := mb.ensureChat(mb.Bot, update)
		if err != nil {
			log.Error().Err(err).Int64("chat_id", update.Message.Chat.ID).Msg("Failed to ensure chat to match its state")
			return false
		}
		return chat.State == chatState && update.Message.Text != "отмена"
	}
	return mb.Bot.RegisterHandlerMatchFunc(matchFunc, f, m...)
}

func callbackDataRegexp(command string) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf("^%s(\n.*)*$", command))
}

// Basic handlers

func (mb *MainBot) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Default handler")

	if update.Message != nil {
		log.Trace().Str("message", update.Message.Text).Msg("Unhandled message")
		if groupName, err := mb.services.Repo.ValidateGroupName(update.Message.Text); err == nil {
			mb.sendQuickGroupSchedule(ctx, groupName, update, b)
		} // Else just ignore message

		return
	}

	if update.CallbackQuery != nil {
		_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Это сообщение больше не поддерживается",
		})
		addContextHandlerError(ctx, err)

		return
	}

	notLogFlag := ctx.Value(noLogFlagContextKey).(*bool)
	*notLogFlag = true
}

func (mb *MainBot) sendQuickGroupSchedule(ctx context.Context, groupName string, update *models.Update, b *bot.Bot) {
	log.Trace().Str("groupName", groupName).Msg("Sending quick group week schedule")

	chat, err := mb.services.Repo.GetChatByTgChatID(update.Message.Chat.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chat from DB")
		return
	}

	group, err := mb.services.Repo.GetGroupByName(groupName)
	if err != nil {
		log.Error().Str("groupName", groupName).Err(err).
			Msg("Failed get group from DB for quick group week schedule")
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
		log.Error().Any("group", group).Err(err).
			Msg("Failed to prepare week schedule data for quick group schedule")
		return
	}

	err = mb.SendWeekScheduleMessages(
		ctx,
		b,
		update.Message.MessageThreadID,
		chat,
		scheduleCfg,
		imageFilename,
		imageData,
	)
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Start handler")
	_, err := mb.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            StartMessage,
	})
	addContextHandlerError(ctx, err)

	if err == nil {
		chat, err := mb.services.Repo.GetChatByTgChatID(update.Message.Chat.ID)
		if err != nil {
			// Error: failed to get chat by chat ID: %!w(<nil>)
			errFormatted := fmt.Errorf("failed to get chat by chat ID: %w", err)
			log.Warn().Err(err).Err(errFormatted).Msg("Failed to get chat by chat ID")
			addContextHandlerError(ctx, errFormatted)
		} else if chat.GroupName == nil {
			mb.groupHandler(ctx, b, update)
		}
	}
}

func (mb *MainBot) helpHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Help handler")
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            HelpMessage,
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) stopHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Stop handler")

	chat, ok := ctx.Value(chatContextKey).(*database.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Не удалось удалить данные чата, попробуйте позже",
		})
		return
	}

	if err := mb.services.Repo.DeleteChat(chat.ID); err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to delete chat: %w", err))
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Не удалось удалить данные чата, попробуйте позже",
		})
		return
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Рассылки остановлены и ваши данные удалены. Спасибо, что пользовались нашим ботом!",
		ReplyMarkup:     models.ReplyKeyboardRemove{RemoveKeyboard: true},
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) deleteHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("Delete handler")

	_, err := bothelpers.DeleteMessageSafely(ctx, b, update.CallbackQuery.Message.Message)
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

	_, err := bothelpers.DeleteMessageSafely(ctx, b, update.Message)
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

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Действие отменено",
		ReplyMarkup:     mainMenuReplyMarkup(update.Message.Chat.ID > 0),
	})
	addContextHandlerError(ctx, err)
}

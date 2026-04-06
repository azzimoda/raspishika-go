package mainbot

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/model"
)

var (
	ErrNoChatContext error = errors.New("failed to get chat from context")
)

func (mb *MainBot) registerHandlers() {
	// Commands
	{
		mb.registerCommandHandler("start", mb.startHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("help", mb.helpHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("stop", mb.stopHandler, mb.checkRegularAccessMiddleware)

		mb.registerCommandHandler("week", mb.weekHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("tomorrow", mb.tomorrowHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("left", mb.todayHandler, mb.checkRegularAccessMiddleware) // TODO: Remove this handler after some time.
		mb.registerCommandHandler("today", mb.todayHandler, mb.checkRegularAccessMiddleware)
		mb.registerCommandHandler("teacher", mb.teacherHandler, mb.checkRegularAccessMiddleware)

		mb.registerCommandHandler("settings", mb.settingsHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("group", mb.groupHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("time", mb.dailyTimeHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("daily_time", mb.dailyTimeHandler, mb.checkConfigAccessMiddleware) // TODO: Remove this handler after some time.
		mb.registerCommandHandler("daily_off", mb.dailyOffHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("reminder_on", mb.reminderOnHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("reminder_off", mb.reminderOffHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("alert_on", mb.alertOnHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("alert_off", mb.alertOffHandler, mb.checkConfigAccessMiddleware)
		mb.registerCommandHandler("access", mb.accessHandler, mb.checkConfigAccessMiddleware)
	}

	// Text messages
	{
		mb.registerTextMessageHandler("неделя", mb.weekHandler, mb.checkRegularAccessMiddleware)
		mb.registerTextMessageHandler("завтра", mb.tomorrowHandler, mb.checkRegularAccessMiddleware)
		mb.registerTextMessageHandler("сегодня", mb.todayHandler, mb.checkRegularAccessMiddleware)

		mb.registerTextMessageHandler("преподаватель", mb.teacherHandler, mb.checkRegularAccessMiddleware)

		mb.Bot.RegisterHandlerMatchFunc(func(update *tgmodels.Update) bool {
			return update.Message != nil && strings.ToLower(update.Message.Text) == "отмена"
		}, mb.textCancelHandler, mb.checkRegularAccessMiddleware)

		mb.registerChatStateHandler(model.ChatStateSelectingGroup, mb.textGroupHandler,
			mb.checkConfigAccessMiddleware)
		mb.registerChatStateHandler(model.ChatStateSelectingTime, mb.textTimeHandler,
			mb.checkConfigAccessMiddleware)
		mb.registerChatStateHandler(model.ChatStateSelectingTeacher, mb.textTeacherNameHandler,
			mb.checkRegularAccessMiddleware)
	}

	// Callback queries
	{
		registerRegularCallbackHandler := func(callbackCommand string, handler bot.HandlerFunc) {
			mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp(callbackCommand),
				handler, mb.checkRegularAccessMiddleware)
		}

		registerRegularCallbackHandler(CallbackCommandDelete, mb.deleteHandler)

		registerRegularCallbackHandler(CallbackCommandSelectTeacher, mb.selectTeacherHandler)

		registerRegularCallbackHandler(CallbackCommandUpdateGroup, mb.updateGroupHandler)
		registerRegularCallbackHandler(CallbackCommandUpdateTeacher, mb.updateTeacherHandler)
		registerRegularCallbackHandler(CallbackCommandUpdateTomorrow, mb.updateTomorrowHandler)
		registerRegularCallbackHandler(CallbackCommandUpdateLeft, mb.updateLeftHandler)

		// Config callbacks
		registerConfigCallbackHandler := func(callbackCommand string, handler bot.HandlerFunc) {
			mb.Bot.RegisterHandlerRegexp(bot.HandlerTypeCallbackQueryData, callbackDataRegexp(callbackCommand),
				handler, mb.checkConfigAccessMiddleware)
		}

		registerConfigCallbackHandler(CallbackCommandDeleteConfig, mb.deleteHandler)

		registerConfigCallbackHandler(CallbackCommandConfigGroup, mb.configGroupHandler)
		registerConfigCallbackHandler(CallbackCommandConfigDailyTime, mb.configDailyTimeHandler)
		registerConfigCallbackHandler(CallbackCommandDailyOff, mb.dailyOffCallbackHandler)
		registerConfigCallbackHandler(CallbackCommandConfigReminder, mb.configReminderHandler)
		registerConfigCallbackHandler(CallbackCommandConfigChange, mb.configChangeHandler)
		registerConfigCallbackHandler(CallbackCommandConfigDarkMode, mb.configDarkModeHandler)
		registerConfigCallbackHandler(CallbackCommandConfigSetAccess, mb.configAccessHandler)
		registerConfigCallbackHandler(CallbackCommandSelectDepartment, mb.selectDepartmentHandler)

		registerConfigCallbackHandler(CallbackCommandSelectDepartment, mb.selectDepartmentHandler)

		registerConfigCallbackHandler(CallbackCommandSetAccess, mb.setAccessHandler)

	}
}

func (mb *MainBot) registerCommandHandler(pattern string, f bot.HandlerFunc, m ...bot.Middleware) string {
	matchFunc := commandMatchFunction(pattern, mb.Me.Username)
	return mb.Bot.RegisterHandlerMatchFunc(matchFunc, f, m...)
}

func commandMatchFunction(pattern string, username string) bot.MatchFunc {
	re := regexp.MustCompile(fmt.Sprintf(`^/%s(@\w+)?(\s[\s\S]+)?$`, pattern))
	return func(update *tgmodels.Update) bool {
		return matchUpdatePatternUsername(update, username, re)
	}
}

func matchUpdatePatternUsername(update *tgmodels.Update, username string, re *regexp.Regexp) bool {
	if update.Message == nil || update.Message.Text == "" {
		return false // Not a text message.
	}

	text := update.Message.Text

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

	if update.Message.Chat.Type == tgmodels.ChatTypeGroup || update.Message.Chat.Type == tgmodels.ChatTypeSupergroup {
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
	chatState model.ChatState,
	f bot.HandlerFunc,
	m ...bot.Middleware,
) string {
	matchFunc := func(update *tgmodels.Update) bool {
		if update.Message == nil {
			return false
		}

		chat, err := mb.ensureChat(mb.Bot, update)
		if err != nil {
			log.Error().Err(err).Int64("chat_id", update.Message.Chat.ID).Msg("Failed to ensure chat to match its state")
			return false
		}
		actualState, expired := chat.State()
		if expired {
			if err := mb.container.Chat.Update(chat.WithState(model.ChatStateDefault)); err != nil {
				log.Error().Err(err).Msg("Failed to reset expired state to default")
			}
		}

		return actualState == chatState && update.Message.Text != "отмена"
	}
	return mb.Bot.RegisterHandlerMatchFunc(matchFunc, f, m...)
}

func callbackDataRegexp(command string) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf("^%s(\n.*)*$", command))
}

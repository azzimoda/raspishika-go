package reporter

import (
	"context"
	"fmt"
	"runtime"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

const (
	logKey      reportKey = "log"
	errKey      reportKey = "err"
	chatIDKey   reportKey = "chat_id"
	usernameKey reportKey = "username"
	debugKey    reportKey = "debug"
)

type Reporter interface {
	Report() ReportConfig
}

func NewReportConfig(bot *bot.Bot, recipientChatID int64) ReportConfig {
	return ReportConfig{bot: bot, recipientChatID: recipientChatID, ctx: defaultContext()}
}

func defaultContext() context.Context {
	ctx := context.Background()

	ctx = context.WithValue(ctx, logKey, false)
	ctx = context.WithValue(ctx, errKey, nil)
	ctx = context.WithValue(ctx, chatIDKey, int64(0))
	ctx = context.WithValue(ctx, usernameKey, "")
	ctx = context.WithValue(ctx, debugKey, make(map[string]any))

	return ctx
}

type ReportConfig struct {
	bot             *bot.Bot
	recipientChatID int64
	ctx             context.Context
}

type reportKey string

func (r ReportConfig) Log() ReportConfig {
	return r.withValue(logKey, true)
}

func (r ReportConfig) Err(err error) ReportConfig {
	return r.withValue(errKey, err)
}

// Chat sets the chat, whose message caused the error. It can be either a Chat object or a chat ID.
func (r ReportConfig) Chat(chatOrID any) ReportConfig {
	if tgChatID, ok := chatOrID.(int64); ok {
		r = r.withValue(chatIDKey, tgChatID)
	} else if chat, ok := chatOrID.(*database.Chat); ok {
		r = r.withValue(chatIDKey, chat.TgChatID).withValue(usernameKey, utils.DerefOrTypeDefault(chat.UserName))
	} else {
		log.Error().Type("type", chatOrID).Any("arg", chatOrID).Msg("Wrong type of chat argument")
	}
	return r
}

// Debug sets a debug object with the given name and value.
func (r ReportConfig) Debug(name string, value any) ReportConfig {
	debugValues, ok := r.ctx.Value(debugKey).(map[string]any)
	if !ok {
		debugValues = make(map[string]any)
		debugValues[name] = value
		r.ctx = context.WithValue(r.ctx, debugKey, debugValues)
	} else {
		debugValues[name] = value
	}
	return r
}

func (r ReportConfig) withValue(key any, value any) ReportConfig {
	r.ctx = context.WithValue(r.ctx, key, value)
	return r
}

// Msg sends a message to the recipient chat with the report information and the given text.
func (rc ReportConfig) Msg(text string) (*models.Message, error) {
	_, file, line, ok := runtime.Caller(1)
	caller := "unknown"
	if ok {
		caller = fmt.Sprintf("%s:%d", file, line)
	}

	// isAdmin := rc.ctx.Value(adminKey).(bool)
	doLog := rc.ctx.Value(logKey).(bool)
	reportErr, ok := rc.ctx.Value(errKey).(error)
	if !ok {
		reportErr = nil
	}
	chatID := rc.ctx.Value(chatIDKey).(int64)
	username := rc.ctx.Value(usernameKey).(string)
	debugObjects := rc.ctx.Value(debugKey).(map[string]any)

	if doLog {
		var logEvent *zerolog.Event
		if reportErr != nil {
			logEvent = log.Error().Err(reportErr)
		} else {
			logEvent = log.Info()
		}
		logEvent.Str("report_caller", caller).Int64("chat_id", chatID).Str("username", username).Msgf("Report: %s", text)
	}

	if rc.bot == nil {
		if log.Logger.GetLevel() == zerolog.TraceLevel {
			log.Warn().Msg("ReportConfig.Send: bot is nil")
		}
		return nil, fmt.Errorf("bot is nil")
	}

	// Assemble the message text.
	msgText := ""

	// Chat
	if chatID != 0 {
		msgText += fmt.Sprintf("`/chat %d` @%s\n", chatID, bot.EscapeMarkdown(username))
	}

	// Error
	if reportErr != nil {
		msgText += fmt.Sprintf("Error: _%s_\n", bot.EscapeMarkdown(reportErr.Error()))
	}

	// Debug objects
	if len(debugObjects) > 0 {
		msgText += "Debug objects:\n```\n"
		for name, value := range debugObjects {
			msgText += fmt.Sprintf("%s=%+v\n", name, value)
		}
		msgText += "```\n"
	}

	// Message text
	msgText += fmt.Sprintf("\n%s", bot.EscapeMarkdown(text))

	msg, err := rc.bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:    rc.recipientChatID,
		Text:      msgText,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		log.Error().Err(err).Str("text", msgText).Msg("Failed to send report message")
	}

	return msg, err
}

// Msgf sends a message to the recipient chat with the report information and the given text.
func (r ReportConfig) Msgf(format string, a ...any) (*models.Message, error) {
	return r.Msg(fmt.Sprintf(format, a...))
}

// Send sends a message to the recipient chat with the report information and an empty text.
// It is a shorthand for Msg("").
func (r ReportConfig) Send() (*models.Message, error) {
	return r.Msg("")
}

package reporter

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

type reportKey string

const (
	logKey            reportKey = "log"
	tempKey           reportKey = "temp"
	errKey            reportKey = "err"
	chatIDKey         reportKey = "chat_id"
	usernameKey       reportKey = "username"
	debugKey          reportKey = "debug"
	escapeMarkdownKey reportKey = "escape_markdown"
)

type Reporter interface{ Report() ReportConfig }

func NewReportConfig(bot *bot.Bot, recipientChatID int64) ReportConfig {
	return ReportConfig{bot: bot, recipientChatID: recipientChatID, Context: defaultContext()}
}

func defaultContext() context.Context {
	ctx := context.Background()

	ctx = context.WithValue(ctx, logKey, false)
	ctx = context.WithValue(ctx, tempKey, false)
	ctx = context.WithValue(ctx, errKey, nil)
	ctx = context.WithValue(ctx, chatIDKey, int64(0))
	ctx = context.WithValue(ctx, usernameKey, "")
	ctx = context.WithValue(ctx, debugKey, make(map[string]any))
	ctx = context.WithValue(ctx, escapeMarkdownKey, true)

	return ctx
}

type ReportConfig struct {
	context.Context
	bot             *bot.Bot
	recipientChatID int64
}

// Log makes the report be printed as log.
func (r ReportConfig) Log() ReportConfig { return r.withValue(logKey, true) }

// Err adds error to report message.
func (r ReportConfig) Err(err error) ReportConfig { return r.withValue(errKey, err) }

// Temp makes the report message be deleted after a minute.
func (r ReportConfig) Temp() ReportConfig { return r.withValue(tempKey, true) }

// Chat sets the chat, whose message caused the error. It can be either a Chat object or a chat ID.
func (r ReportConfig) Chat(chatOrID any) ReportConfig {
	if tgChatID, ok := chatOrID.(int64); ok {
		r = r.withValue(chatIDKey, tgChatID)
	} else if chat, ok := chatOrID.(*models.Chat); ok {
		r = r.withValue(chatIDKey, chat.TgChatID).withValue(usernameKey, utils.DerefOrTypeDefault(chat.UserName))
	} else {
		log.Error().Type("type", chatOrID).Any("arg", chatOrID).Msg("Wrong type of chat argument")
	}
	return r
}

// Debug sets a debug object with the given name and value.
func (r ReportConfig) Debug(name string, value any) ReportConfig {
	debugValues, ok := r.Value(debugKey).(map[string]any)
	if !ok {
		debugValues = make(map[string]any)
		debugValues[name] = value
		r.Context = context.WithValue(r.Context, debugKey, debugValues)
	} else {
		debugValues[name] = value
	}
	return r
}

// MD disables markdown escaping in the report message.
//
// If the string provided to any of finalizing method contains wrong markdown formatting,
// it may cause Telegram API errors.
func (r ReportConfig) MD() ReportConfig {
	return r.withValue(escapeMarkdownKey, false)
}

func (r ReportConfig) withValue(key any, value any) ReportConfig {
	r.Context = context.WithValue(r.Context, key, value)
	return r
}

// Msg sends a message to the recipient chat with the report information and the given text.
func (rc ReportConfig) Msg(text string) (*Report, error) {
	_, file, line, ok := runtime.Caller(1)
	caller := "unknown"
	if ok {
		caller = fmt.Sprintf("%s:%d", file, line)
	}

	doLog := rc.Value(logKey).(bool)
	isTemp := rc.Value(tempKey).(bool)
	reportErr, ok := rc.Value(errKey).(error)
	if !ok {
		reportErr = nil
	}
	chatID := rc.Value(chatIDKey).(int64)
	username := rc.Value(usernameKey).(string)
	debugObjects := rc.Value(debugKey).(map[string]any)
	doEscapeMarkdown := rc.Value(escapeMarkdownKey).(bool)

	// NOTE: Log the report when it is temporary to not lose information.
	if doLog || isTemp {
		var logEvent *zerolog.Event
		if reportErr != nil {
			logEvent = log.Error().Err(reportErr)
		} else {
			logEvent = log.Info()
		}
		for key, value := range debugObjects {
			logEvent.Any(key, value)
		}
		logEvent.Str("report_caller", caller).Int64("chat_id", chatID).Str("username", username).
			Msgf("Report: %s", text)
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
		msgText += fmt.Sprintf("Error:\n```\n%s\n```\n", bot.EscapeMarkdown(reportErr.Error()))
	}

	// Debug objects
	if len(debugObjects) > 0 {
		msgText += "\n```\n"
		for name, value := range debugObjects {
			msgText += fmt.Sprintf("%s=%+v\n", name, value)
		}
		msgText += "```\n"
	}

	// Message text
	if doEscapeMarkdown {
		msgText += bot.EscapeMarkdownUnescaped(text)
	} else {
		msgText += text
	}

	// Send the message.
	msg, err := rc.bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:    rc.recipientChatID,
		Text:      msgText,
		ParseMode: tgmodels.ParseModeMarkdown,
	})
	if err != nil {
		rc.bot.SendMessage(context.Background(), &bot.SendMessageParams{
			ChatID:    rc.recipientChatID,
			Text:      fmt.Sprintf("Failed to send report:\n```\n%s\n```", err),
			ParseMode: tgmodels.ParseModeMarkdown,
		})
		log.Error().Err(err).Str("text", msgText).Msg("Failed to send report message")
	}
	if err == nil && isTemp {
		log.Trace().Msgf("The report message will be deleted after a minute...")
		go func() {
			time.Sleep(time.Minute)
			rc.bot.DeleteMessage(context.Background(), &bot.DeleteMessageParams{
				ChatID:    rc.recipientChatID,
				MessageID: msg.ID,
			})
			log.Trace().Msgf("The report message is deleted.")
		}()
	}

	return &Report{rc, msg}, err
}

// Msgf sends a message to the recipient chat with the report information and the given text.
func (r ReportConfig) Msgf(format string, a ...any) (*Report, error) {
	return r.Msg(fmt.Sprintf(format, a...))
}

// Send sends a message to the recipient chat with the report information and an empty text.
// It is a shorthand for Msg("").
func (r ReportConfig) Send() (*Report, error) {
	return r.Msg("")
}

type Report struct {
	ReportConfig
	Message *tgmodels.Message
}

func (r *Report) RemoveMessage() (isDeleted bool, err error) {
	isDeleted, err = r.bot.DeleteMessage(context.Background(), &bot.DeleteMessageParams{
		ChatID:    r.recipientChatID,
		MessageID: r.Message.ID,
	})
	log.Trace().Msgf("The report message is deleted")
	return
}

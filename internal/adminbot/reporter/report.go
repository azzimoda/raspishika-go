package reporter

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Reporter interface {
	Report() ReportConfig
}

type ReportConfig struct {
	api             *tgbotapi.BotAPI
	admin           bool
	recipientChatID int64 // RecipientChatID is the ID of the chat, where the report should be sent.
	chatID          int64 // ChatID is the ID of the chat, whose message caused the error. Optional.
	username        string
	err             error
}

func (r ReportConfig) Admin() ReportConfig {
	r.admin = true
	return r
}

func (r ReportConfig) Chat(chatOrID any) ReportConfig {
	if chatID, ok := chatOrID.(int64); ok {
		r.chatID = chatID
	} else if chat, ok := chatOrID.(*database.Chat); ok {
		r.chatID = chat.ChatID
		r.username = utils.DerefOrTypeDefault(chat.UserName)
	} else {
		log.Error().Any("arg", chatOrID).Msg("Wrong type of chat argument")
	}
	return r
}

func (r ReportConfig) Err(err error) ReportConfig {
	r.err = err
	return r
}

func (r ReportConfig) Send(text string) {
	if r.api == nil {
		if log.Logger.GetLevel() <= zerolog.DebugLevel {
			log.Warn().Msg("ReportConfig.Send: bot is nil")
		}
		return
	}

	msgText := ""
	if r.chatID != 0 {
		msgText += fmt.Sprintf("\n`/chat %d` @%s", r.chatID, tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, r.username))
	}
	if r.err != nil {
		msgText += fmt.Sprintf("\nError: _%s_", tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, r.err.Error()))
	}
	msgText += fmt.Sprintf("\n\n%s", tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, text))

	msg := tgbotapi.NewMessage(r.recipientChatID, msgText)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err := r.api.Send(msg)
	if err != nil {
		log.Error().Err(err).Str("text", msgText).Msg("Failed to send report message")
	}
}

func (r ReportConfig) Sendf(format string, a ...any) {
	r.Send(fmt.Sprintf(format, a...))
}

func NewReportConfig(api *tgbotapi.BotAPI, recipientChatID int64) ReportConfig {
	return ReportConfig{api: api, recipientChatID: recipientChatID}
}

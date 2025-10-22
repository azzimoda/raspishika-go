package reporter

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/pkg/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

type Reporter interface {
	Report() ReportConfig
}

type ReportConfig struct {
	api             *tgbotapi.BotAPI
	recipientChatID int64 // RecipientChatID is the ID of the chat, where the report should be sent.
	chatID          int64 // ChatID is the ID of the chat, whose message caused the error. Optional.
	err             error
}

func (r ReportConfig) Chat(chatID int64) ReportConfig {
	r.chatID = chatID
	return r
}

func (r ReportConfig) Err(err error) ReportConfig {
	r.err = err
	return r
}

func (r ReportConfig) Send(text string) {
	if r.api == nil {
		log.Trace().Msg("ReportConfig.Send: bot is nil")
		return
	}

	msgText := ""
	if r.chatID != 0 {
		msgText += fmt.Sprintf("\n`/chat %d`", r.chatID)
	}
	if r.err != nil {
		msgText += fmt.Sprintf("\nError: _%s_", utils.EscapeMarkdown(r.err.Error()))
	}
	msgText += fmt.Sprintf("\n\n%s", text)

	msg := tgbotapi.NewMessage(r.recipientChatID, msgText)
	msg.ParseMode = "MarkdownV2"
	_, err := r.api.Send(msg)
	if err != nil {
		log.Error().Err(err).Str("text", msgText).Msg("Failed to send report message")
	}
}

func NewReportConfig(api *tgbotapi.BotAPI, recipientChatID int64) ReportConfig {
	return ReportConfig{api: api, recipientChatID: recipientChatID}
}

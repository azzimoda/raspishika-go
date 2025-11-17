package reporter

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

type Reporter interface {
	Report() ReportConfig
}

type ReportConfig struct {
	bot               *bot.Bot
	admin             bool
	recipientTgChatID int64 // RecipientChatID is the ID of the chat, where the report should be sent.
	tgChatID          int64 // tgChatID is the ID of the chat, whose message caused the error. Optional.
	username          string
	err               error
}

func (r ReportConfig) Admin() ReportConfig {
	r.admin = true
	return r
}

func (r ReportConfig) Chat(chatOrID any) ReportConfig {
	if tgChatID, ok := chatOrID.(int64); ok {
		r.tgChatID = tgChatID
	} else if chat, ok := chatOrID.(*database.Chat); ok {
		r.tgChatID = chat.TgChatID
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

func (rc ReportConfig) Send(text string) {
	if rc.bot == nil {
		if log.Logger.GetLevel() == zerolog.TraceLevel {
			log.Warn().Msg("ReportConfig.Send: bot is nil")
		}
		return
	}

	msgText := ""
	if rc.tgChatID != 0 {
		msgText += fmt.Sprintf("\n`/chat %d` @%s", rc.tgChatID, bot.EscapeMarkdown(rc.username))
	}
	if rc.err != nil {
		msgText += fmt.Sprintf("\nError: _%s_", bot.EscapeMarkdown(rc.err.Error()))
	}
	msgText += fmt.Sprintf("\n\n%s", bot.EscapeMarkdown(text))

	_, err := rc.bot.SendMessage(context.Background(), &bot.SendMessageParams{
		ChatID:    rc.recipientTgChatID,
		Text:      msgText,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		log.Error().Err(err).Str("text", msgText).Msg("Failed to send report message")
	}
}

func (r ReportConfig) Sendf(format string, a ...any) {
	r.Send(fmt.Sprintf(format, a...))
}

func NewReportConfig(bot *bot.Bot, recipientChatID int64) ReportConfig {
	return ReportConfig{bot: bot, recipientTgChatID: recipientChatID}
}

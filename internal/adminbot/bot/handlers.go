package bot

import (
	"fmt"
	"strconv"

	"github.com/azzimoda/raspishika-go/pkg/utils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (b *AdminBot) OnUpdate(update tgbotapi.Update) {
	var err error
	switch {
	case update.Message != nil:
		err = b.onMessage(update.Message)
	default:
		log.Warn().Msgf("Unknown update type")
	}
	if err != nil {
		log.Error().Err(err).Msg("Error handling update")
		b.Report().Err(err).Send("[ADMINBOT] Error handling update")
	}
}

func (b *AdminBot) onMessage(msg *tgbotapi.Message) error {
	if msg.Chat.ID != b.Config.Telegram.AdminID {
		return fmt.Errorf("access denied to user %d", msg.From.ID)
	}

	if msg.IsCommand() {
		return b.onCommand(msg)
	} else {
		return b.onText(msg)
	}
}

func (b *AdminBot) onCommand(msg *tgbotapi.Message) error {
	switch msg.Command() {
	case "chat":
		return b.onChat(msg)
	case "group":
		return b.onGroup(msg)
	case "chats":
		return b.onChats(msg)
	default:
		b.api.Send(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID))
		log.Warn().Msgf("Unknown command: %s", msg.Command())
	}

	return nil
}

func (b *AdminBot) onText(msg *tgbotapi.Message) error {
	if chatID, err := strconv.ParseInt(msg.Text, 10, 64); err == nil {
		return b.sendChatReport(chatID, msg)
	}

	if group, err := utils.ValidateGroupNameFormat(msg.Text); err == nil {
		return b.sendGroupReport(group, msg)
	}

	log.Warn().Msgf("Invalid message: %s", msg.Text)
	return nil
}

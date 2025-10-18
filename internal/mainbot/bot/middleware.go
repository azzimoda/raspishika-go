package bot

import (
	"github.com/azzimoda/raspishika-go/internal/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/rs/zerolog/log"
)

func (b *Bot) ApplyMiddleware(update tgbotapi.Update, repo *database.Repository) bool {
	if update.Message != nil {
		chatID := update.Message.Chat.ID
		username := update.Message.Chat.UserName
		if _, err := repo.CreateOrUpdateChat(chatID, username); err != nil {
			log.Error().Err(err).Int64("chatID", chatID).Str("username", username).
				Msg("Failed to create or update chat")
			return false
		}
	}

	return true
}

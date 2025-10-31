package bot

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
)

func (b *Bot) ApplyMiddleware(update tgbotapi.Update, repo *database.Repository) bool {
	if update.Message != nil {
		chatID := update.Message.Chat.ID
		username := update.Message.Chat.UserName
		chat, created, err := repo.CreateOrUpdateChat(chatID, username)
		if err != nil {
			log.Error().Err(err).Int64("chatID", chatID).Str("username", username).
				Msg("Failed to create or update chat")
			return false
		}

		if created {
			log.Trace().Int64("chatID", chatID).Str("username", username).Msg("New chat registered")
			b.Reporter.Report().Chat(chatID).Send("New chat registered")
			go func() {
				time.Sleep(20 * time.Second)
				if chat, err := repo.GetChatByChatID(chatID); err == nil && chat.GroupName != nil {
					b.Reporter.Report().Chat(chatID).Sendf("Chat configured group %s", *chat.GroupName)
				}
			}()
		}

		isConfigCommand := isConfigCommand(update.Message.Command())
		isAdmin := b.IsAdmin(update.Message.Chat.ID, update.Message.From.ID)
		if !chat.IsPrivate() && (chat.Access == 1 && isConfigCommand && !isAdmin || chat.Access == 2 && !isAdmin) {
			// User is restricted to use given command when chat is a supergroup and:
			// - chat access is 1, the command is config, the user is not admin of the chat;
			// - chat access is 2 and the user is not admin of the chat.
			return false
		}
	}

	return true
}

func (b *Bot) IsAdmin(chatID, userID int64) bool {
	chatMember, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{ChatID: chatID, UserID: userID},
	})
	if err != nil {
		log.Error().Err(err).Int64("chatID", chatID).Int64("userID", userID).Msg("Failed to get chat member; fallback to admin")
		return true
	}
	return chatMember.IsAdministrator() || chatMember.IsCreator()
}

func isConfigCommand(text string) bool {
	switch text {
	case "settings", "group", "daily_time", "daily_off", "remainder_on", "remainder_off", "access":
		return true
	default:
		return false
	}
}

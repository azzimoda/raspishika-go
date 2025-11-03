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
			b.Reporter.Report().Chat(chat).Send("New chat registered")
			go func() {
				time.Sleep(20 * time.Second)
				if chat, err := repo.GetChatByChatID(chatID); err == nil && chat.GroupName != nil {
					b.Reporter.Report().Chat(chatID).Sendf("Chat configured group %s", *chat.GroupName)
				}
			}()
		}

		if !chat.IsPrivate() && !b.checkAccess(chatID, update.Message.From.ID, chat.Access, update.Message.Command()) {
			return false
		}
	}

	if update.CallbackQuery != nil {
		chatID := update.CallbackQuery.Message.Chat.ID
		callbackCommand := ParseCallbackData(update.CallbackQuery.Data)

		chat, err := repo.GetChatByChatID(chatID)
		if err == nil {
			if !chat.IsPrivate() && !b.checkAccess(
				chatID,
				update.CallbackQuery.From.ID,
				chat.Access,
				callbackCommand.Command,
			) {
				return false
			}
		}
		log.Error().Err(err).Int64("chatID", chatID).Msg("Failed to get chat")
	}

	return true
}

// checkAccess checks if user is allowed to use given command in chat.
//
// User is restricted to use given command when chat is a supergroup and:
// - chat access is 1, the command is config, the user is not admin of the chat;
// - chat access is 2 and the user is not admin of the chat.
func (b *Bot) checkAccess(chatID, userID int64, accessLevel int, command string) bool {
	isConfigCommand := isConfigCommand(command)
	isAdmin := b.IsAdmin(chatID, userID)
	if accessLevel == 1 && isConfigCommand && !isAdmin || accessLevel == 2 && !isAdmin {
		return false
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
		// Commands
		return true
	case "delete_config", "config_daily_time", "config_reminder", "config_access", "select_department", "set_access":
		// Callback queries
		return true
	default:
		return false
	}
}

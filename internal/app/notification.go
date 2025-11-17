package app

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (a *App) SendNotification(s string) {
	chats, err := a.Services.Repo.GetChats()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get chats")
	}

	log.Info().Int("chatsCount", len(chats)).Msg("Sending notification")

	okCount := 0
	for _, chat := range chats {
		err := sendNotification(a.MainBot.API(), &chat, s)
		if err != nil {
			log.Error().Err(err).
				Int64("tgChatID", chat.TgChatID).
				Str("username", utils.DerefOrTypeDefault(chat.UserName)).
				Msg("Failed to send notification")
			continue
		}

		okCount++
		log.Debug().
			Int64("tgChatID", chat.TgChatID).
			Str("username", utils.DerefOrTypeDefault(chat.UserName)).
			Msg("Notification sent")
	}

	log.Info().Int("okCount", okCount).Int("errCount", len(chats)-okCount).Msg("Notification sent")
}

func sendNotification(api *tgbotapi.BotAPI, chat *database.Chat, s string) error {
	_, err := api.Send(tgbotapi.NewMessage(chat.TgChatID, s))
	return err
}

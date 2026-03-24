package app

import (
	"context"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/pkg/refutil"
)

const notificationWorkers = 8

func (a *App) Notify(s string) {
	chats, err := models.GetChats(a.services.Repository.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get chats")
	}

	log.Info().Int("chatCount", len(chats)).Int("workers", notificationWorkers).Msg("Sending notification...")

	var wg sync.WaitGroup
	results := make(chan int, 10)

	go func() {
		for _, chat := range chats {
			wg.Go(func() {
				err := sendNotification(a.mainBot.Bot, &chat, s)
				if err != nil {
					log.Error().Err(err).
						Any("tgChatID", chat.TgChatID).
						Any("username", refutil.DerefOrTypeDefault(chat.UserName)).
						Msg("Failed to send notification")
					results <- 0
					return
				}

				log.Debug().
					Any("tgChatID", chat.TgChatID).
					Any("username", refutil.DerefOrTypeDefault(chat.UserName)).
					Msg("Notification sent")
				results <- 1
			})
		}

		wg.Wait()
		close(results)
	}()

	okCount := 0
	for r := range results {
		okCount += r
	}

	log.Info().Int("okCount", okCount).Int("errCount", len(chats)-okCount).Msg("Notification sent")
}

func sendNotification(b *bot.Bot, chat *models.Chat, text string) error {
	_, err := b.SendMessage(context.Background(), &bot.SendMessageParams{ChatID: chat.TgChatID, Text: text})
	return err
}

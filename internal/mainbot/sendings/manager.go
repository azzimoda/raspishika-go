package sendings

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot"
	"github.com/azzimoda/raspishika-go/internal/services"
)

func NewSendingManager(bot *mainbot.MainBot, services *services.Services) *SendingManager {
	return &SendingManager{cron: cron.New(), bot: bot, services: services}
}

type SendingManager struct {
	cron     *cron.Cron
	bot      *mainbot.MainBot
	services *services.Services
}

func (sm *SendingManager) Start() {
	sm.cron.Start()
}

func (sm *SendingManager) ScheduleDailySending() error {
	_, err := sm.cron.AddFunc("* * * * *", func() {
		go sm.processDailySending(time.Now())
	})
	return err
}

func (sm *SendingManager) SchedulePairSending() error {
	times := [][2]int{
		{7, 45},  // 08:00
		{9, 30},  // 09:45
		{11, 15}, // 11:30
		// Big break, 40 minutes.
		{13, 30}, // 13:45
		{15, 15}, // 15:30
		{17, 00}, // 17:15
		{18, 45}, // 19:00
	}
	for _, t := range times {
		h := t[0]
		m := t[1]
		_, err := sm.cron.AddFunc(fmt.Sprintf("%d %d * * 1-6", m, h), func() {
			go sm.processPairSending(time.Now())
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// handleTelegramAPIError handles errors returned by Telegram API.
// Returns an error if the error is not recoverable.
func handleTelegramAPIError(services *services.Services, chat *database.Chat, err error) error {
	if err == nil {
		return nil
	}

	// if errors.Is(err, bot.ErrorBadRequest) {
	// 	// Handle the ErrorBadRequest (400) case here
	// }

	// if errors.Is(err, bot.ErrorUnauthorized) {
	// 	// Handle the ErrorUnauthorized (401) case here
	// }

	if errors.Is(err, bot.ErrorForbidden) {
		// Handle the ErrorForbidden (403) case here
		log.Warn().Err(err).Msg("Telegram API error: Forbidden; deactivating sendings for chat...")
		chat.DailySendingTime = nil
		chat.PairSending = false
		if err := services.Repo.UpdateChat(chat); err != nil {
			return fmt.Errorf("failed to deactivate forbidden sendings for chat: %w", err)
		}
		return nil
	}

	// if errors.Is(err, bot.ErrorNotFound) {
	// 	// Handle the ErrorNotFound (404) case here
	// }

	// if errors.Is(err, bot.ErrorConflict) {
	// 	// Handle the ErrorConflict (409) case here
	// }

	if bot.IsMigrateError(err) {
		// Handle the MigrateError (420) case here
		migrateErr := err.(*bot.MigrateError)
		log.Warn().
			Err(migrateErr).
			Int("migrate_to_chat_id", migrateErr.MigrateToChatID).
			Msg("Telegram API error: MigrateError")

		if c, err := services.Repo.GetChatByTgChatID(int64(migrateErr.MigrateToChatID)); err == nil {
			log.Warn().
				Int64("tgChatID", chat.TgChatID).
				Int64("migrateToChatID", int64(migrateErr.MigrateToChatID)).
				Msg("Chat already exists, deleting old chat...")
			if err := services.Repo.DeleteChat(c.ID); err != nil {
				return fmt.Errorf("failed to delete old chat: %w", err)
			}
		} else {
			if err := services.Repo.UpdateChatTgChatID(chat.ID, int64(migrateErr.MigrateToChatID)); err != nil {
				log.Error().
					Err(err).
					Int64("tgChatID", chat.TgChatID).
					Int("migrateToChatID", migrateErr.MigrateToChatID).
					Msg("Failed to update chat ID")
				return fmt.Errorf("failed to update chat ID: %w", err)
			}
		}

		log.Info().
			Int64("tgChatID", chat.TgChatID).
			Int("migrateToChatID", migrateErr.MigrateToChatID).
			Msg("Chat ID migration applied successfully")
		return nil
	}

	if bot.IsTooManyRequestsError(err) {
		// Handle the TooManyRequestsError (429) case here
		retryAfter := err.(*bot.TooManyRequestsError).RetryAfter
		fmt.Println("Received TooManyRequestsError with retry_after:", retryAfter)
		// NOTE: It would be better to implement timeout and retry logic here. But it seems to work fine without it.
	}

	return fmt.Errorf("unhandled Telegram API error: %w", err)
}

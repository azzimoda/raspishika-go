package broadcast

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service/browser"
	"github.com/azzimoda/raspishika-go/internal/service/schedule"
)

func NewNotificationService(
	scheduleService *schedule.ScheduleManager,
	container repository.Container,
	reporter reporter.Reporter,
) *BroadcastService {
	return &BroadcastService{
		scheduleService: scheduleService,
		Container:       container,
		Reporter:        reporter,
		cron:            cron.New(),
	}
}

type BroadcastService struct {
	scheduleService *schedule.ScheduleManager
	*browser.BrowserService
	repository.Container
	reporter.Reporter
	*bot.Bot
	cron *cron.Cron
}

func (s *BroadcastService) Start() {
	s.cron.Start()
}

func (s *BroadcastService) ScheduleDaily() error {
	_, err := s.cron.AddFunc("* * * * *", func() {
		go s.processDaily(time.Now())
	})
	return err
}

func (s *BroadcastService) SchedulePairNotification() error {
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
		h, m := t[0], t[1]
		_, err := s.cron.AddFunc(fmt.Sprintf("%d %d * * 1-6", m, h), func() {
			go s.processPairNotification(time.Now())
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *BroadcastService) RunChangeAlertNotifier(ctx context.Context) {
	halfInterval := config.ScheduleUpdateMonitorInterval() / 2
	log.Info().Dur("halfInterval", halfInterval).Msg("Change alert notifier will start in half interval")
	time.Sleep(halfInterval)

	log.Info().Msg("Change alert notifier started")
	for {
		if s.Bot == nil {
			log.Warn().Msg("Bot is not set yet")
			time.Sleep(5 * time.Second)
			continue
		}

		select {
		case <-ctx.Done():
			log.Info().Msg("Schedule change alert notifier stopped")
			return
		default:
			s.processChangeAlert(ctx)
			time.Sleep(config.ScheduleUpdateMonitorInterval())
		}
	}
}

func (s *BroadcastService) handleAPIError(ctx context.Context, chat *model.Chat, err error) error {
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
		// Check if bot is kicked

		me, err := s.GetMe(ctx)
		if err != nil {
			return fmt.Errorf("failed to get bot's chat member")
		}

		ctx := context.Background()
		botMember, err := s.GetChatMember(ctx, &bot.GetChatMemberParams{ChatID: chat.ID, UserID: me.ID})
		if err != nil {
			return fmt.Errorf("failed to get bot's chat member")
		}

		switch botMember.Type {
		case tgmodels.ChatMemberTypeBanned:
			// Bot was kicked
			if err := s.Chat.Delete(chat); err != nil {
				log.Error().Err(err).Msg("Failed to delete fobidden chat")
				return err
			}

			s.Report().Log().Chat(chat).Msg("Bot was kicked from chat :[")
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
		migrateErr := err.(*bot.MigrateError)
		log.Warn().Err(migrateErr).
			Int("migrate_to_chat_id", migrateErr.MigrateToChatID).
			Msg("Chat migrated to new ID")

		if _, err := s.Chat.GetByChatID(model.ChatID(migrateErr.MigrateToChatID)); err != nil {
			log.Warn().
				Int64("old_id", chat.TgChatID.Int64()).
				Int64("new_id", int64(migrateErr.MigrateToChatID)).
				Msg("Chat already exists, deleting old chat...")
			if err := s.Chat.Delete(chat); err != nil {
				return fmt.Errorf("failed to delete old chat: %w", err)
			}
		} else {
			chat.TgChatID = model.ChatID(int64(migrateErr.MigrateToChatID))
			if err := s.Chat.Update(chat); err != nil {
				log.Error().Err(err).
					Int64("old_id", chat.TgChatID.Int64()).
					Int64("new_id", int64(migrateErr.MigrateToChatID)).
					Msg("Failed to update migrated chat ID")
				return fmt.Errorf("faield to update migrated chat ID: %w", err)
			}
		}

		log.Info().
			Int64("tgChatID", chat.TgChatID.Int64()).
			Int("migrateToChatID", migrateErr.MigrateToChatID).
			Msg("Chat ID migration applied successfully")
		return nil
	}

	// if bot.IsTooManyRequestsError(err) {
	// 	// Handle the TooManyRequests (429) case here
	// }

	return err
}

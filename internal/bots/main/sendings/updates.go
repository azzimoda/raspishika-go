package sendings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	mainbot "github.com/azzimoda/raspishika-go/internal/bots/main"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/models"
	schedulemanager "github.com/azzimoda/raspishika-go/internal/services/schedule/manager"
)

func (sm *SendingManager) RunUpdatesNotifier(ctx context.Context) {
	log.Info().Msg("Updates notifier started")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var updates = make(chan *models.ScheduleChange)

	go sm.RunUpdateMonitor(ctx, updates)
	log.Debug().Msg("Update monitor started")

	log.Debug().Msg("Update worker started")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Updates notifier stopped")
			return
		case change := <-updates:
			log.Debug().Str("groupName", change.Old.Config.Group.GroupName).Msg("Received schedule update")
			var err error
			elapsed := measureTime(func() {
				err = sm.SendUpdateNotificationForGroup(ctx, change)
			})
			if err != nil {
				sm.services.Reporter.Report().Log().Err(err).
					Debug("config", change.New.Config).
					Debug("elapsed", elapsed).
					Msg("Errors while update sendings")
			}
		default:
			time.Sleep(time.Second)
		}
	}
}

func (sm *SendingManager) SendUpdateNotificationForGroup(ctx context.Context, change *models.ScheduleChange) error {
	log.Trace().Any("config", change.New.Config).Msg("Sending update notificatin for group...")
	chats, err := models.GetChatsByGroup(sm.services.Repo.DB, change.New.Config.Group.GroupName)
	if err != nil {
		return err
	}

	var errs []error
	for _, chat := range chats {
		if err := sm.sendUpdateNotification(ctx, change, &chat); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (sm *SendingManager) sendUpdateNotification(
	ctx context.Context,
	change *models.ScheduleChange,
	chat *models.Chat,
) error {
	var errs []error

	imageFileName, imageData, err := sm.bot.PrepareScheduleImage(change.New.Config)
	if err != nil {
		return fmt.Errorf("failed to prepare schedule image: %w", err)
	}

	text := change.String()
	sm.services.Reporter.Report().MD().Msgf("Schedule change:\n%s", text)

	if _, err = sm.bot.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chat.TgChatID,
		Text:      text,
		ParseMode: tgmodels.ParseModeMarkdown,
	}); err != nil {
		return fmt.Errorf("failed to send schedule update notification: %w", err)
	}

	if err = sm.bot.SendSchedulePhoto(ctx, sm.bot.Bot, chat, 0, imageFileName, imageData,
		mainbot.WeekScheduleMarkup(change.New.Config),
	); err != nil {
		sm.bot.Bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chat.TgChatID,
			Text:   "Не удалось отправить изображение расписания",
		})
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (sm *SendingManager) RunUpdateMonitor(ctx context.Context, updates chan<- *models.ScheduleChange) {
	interval := config.UpdateNotificationInterval()
	log.Debug().Dur("interval", interval).Msg("Schedule updating interval")
	for {
		select {
		case <-ctx.Done():
			close(updates)
			log.Debug().Msg("Update monitor stopped")
			return
		default:
		}

		// Fetch groups
		groups, err := models.GetMonitoredGroups(sm.services.Repo.DB)
		if err != nil {
			sm.services.Reporter.Report().Log().Err(err).
				Msg("Failed to get chats with update notification enabled grouped by student group")
			continue
		}
		chatCount, err := models.GetChatsCountWithUpdateSendingEnabled(sm.services.Repo.DB)
		if err != nil {
			sm.services.Reporter.Report().Log().Err(err).Msg("Failed to get chats with update notification enabled")
			chatCount = -1
		}
		log.Debug().Int("groupCount", len(groups)).Int("chatsCount", chatCount).Msg("Checking updates...")

		changesDetected := 0

		// Fetch schedules
		var errs []error
		elapsed := measureTime(func() {
			for _, group := range groups {
				conf := models.GroupScheduleConfig(&group)
				oldRawSchedule, err := sm.services.ScheduleMan.GetCache(sm.services.Repo, conf)
				if errors.Is(err, schedulemanager.ErrNoCache) {
					log.Warn().Err(err).Any("config", conf).Msg("No cache for the schedule config; just updating...")
					_, err := sm.services.ScheduleMan.UpdateCache(sm.services.Repo, sm.services.Browser, conf)
					errs = append(errs, err)
					continue
				}
				if err != nil {
					log.Error().Err(err).Any("config", conf).Msg("Failed to get schedule cache")
					errs = append(errs, err)
					continue
				}

				newRawSchedule, err := sm.services.ScheduleMan.UpdateCache(sm.services.Repo, sm.services.Browser, conf)
				if err != nil {
					log.Error().Err(err).Any("config", conf).Msg("Failed to update schedule cache")
					errs = append(errs, err)
					continue
				}

				// Check if schedule changed
				change := models.NewScheduleChange(oldRawSchedule.Transform(), newRawSchedule.Transform())
				if len(change.Diffs()) > 0 {
					log.Debug().Any("config", conf).Msg("Schedule change detected")
					// Send the change to channel
					changesDetected++
					updates <- change
				}
			}
		})

		errSending := errors.Join(errs...)
		errStr := ""
		if errSending != nil {
			errStr = errSending.Error()
		}

		// Log statistics
		elapsedPerChat := elapsed / time.Duration(chatCount)
		elapsedPerGroup := elapsed / time.Duration(len(groups))
		log.Debug().Int("groupCount", len(groups)).Dur("elapsed", elapsed).
			Dur("elapsedPerGroup", elapsedPerGroup).
			Msgf("Monitored groups schedules updated in %v (%v/group)", elapsed, elapsedPerGroup)
		if len(groups) > 0 && changesDetected > 0 {
			if err := models.InsertSendingLog(sm.services.Repo.DB, models.SendingLog{
				Kind:    models.SendingLogUpdate,
				Chats:   chatCount,
				Groups:  len(groups),
				Elapsed: int(elapsed.Milliseconds()),
				Fails:   len(errs),
				Errors:  errStr,
			}); err != nil {
				log.Error().Err(err).Msg("Failed to insert sending log")
			}

			log.Info().
				Dur("elapsed", elapsed).
				Dur("elapsedPerChat", elapsedPerChat).
				Dur("elapsedPerGroup", elapsedPerGroup).
				Int("chats", chatCount).
				Int("groups", len(groups)).
				Int("changes", changesDetected).
				Int("errs", len(errs)).
				Msgf("Monitored groups schedules updated in %v (%v/group)", elapsed, elapsedPerGroup)
		}

		err = errors.Join(errs...)
		if err != nil {
			sm.services.Reporter.Report().Log().Err(err).Debug("groupCount", len(groups)).Debug("elapsed", elapsed).
				Msg("Errors while fetchig updates for monitored groups")
		} else if elapsed.Minutes() > 1 || elapsedPerGroup.Seconds() > 10 {
			sm.services.Reporter.Report().Log().Debug("groupCount", len(groups)).Debug("elapsed", elapsed).
				Msg("Updating monitored groups schedules took too long")
		}

		time.Sleep(interval)
	}
}

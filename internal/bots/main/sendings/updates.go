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

/*
 * Update notifier algorithm:
 * 1. Collect chats with updates notifications on
 * 2. Group them by configured student group
 * 3. Check updates for each student group
 * 4. Send update reports
 * 5. Repeat until shutdown
 *
 * How should the schedule fetching algorithm change:
 * - How it was:
 *   1. Check cache
 *     - If it ok, return it
 *     - Else...
 *   2. Fetch schedule
 *   3. Update cache
 *   4. Return the new cache
 * - How will be:
 *   1. Check whether the requested group is on update monitoring
 *     (find in DB a chat with this group configured and updates notifier enabled)
 *     - If it is on monitoring, just return current cache
 *     - Else just do the same algorithm
 */

func (sm *SendingManager) RunUpdatesNotifier(ctx context.Context) {
	log.Info().Msg("Updates notifier started")

	var updates = make(chan *models.ScheduleChange)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go sm.RunUpdateMonitor(ctx, updates)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Updates notifier stopped")
			return
		case change := <-updates:
			log.Info().Str("groupName", change.Old.Config.Group.GroupName).Msgf("Received schedule update")
			if err := sm.SendUpdateNotificationForGroup(ctx, change); err != nil {
				sm.services.Reporter.Report().Log().Err(err).Msg("Errors while update sendings")
			}
		default:
			time.Sleep(time.Second)
		}
	}
}

func (sm *SendingManager) SendUpdateNotificationForGroup(ctx context.Context, change *models.ScheduleChange) error {
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

	if _, err = sm.bot.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chat.TgChatID,
		Text:      bot.EscapeMarkdownUnescaped(change.String()),
		ParseMode: tgmodels.ParseModeMarkdown,
	}); err != nil {
		return fmt.Errorf("failed to send schedule update notification: %w", err)
	}

	if err = sm.bot.SendSchedulePhoto(
		ctx,
		sm.bot.Bot,
		chat,
		0,
		imageFileName,
		imageData,
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
	for {
		select {
		case <-ctx.Done():
			close(updates)
			return
		default:
		}

		groups, err := models.GetMonitoredGroups(sm.services.Repo.DB)
		if err != nil {
			sm.services.Reporter.Report().Log().Err(err).
				Msg("Failed to get chats with update notification enabled grouped by student group")
			continue
		}
		var errs []error
		for _, group := range groups {
			conf := models.GroupScheduleConfig(&group)
			oldRawSchedule, err := sm.services.ScheduleManager.GetCache(sm.services.Repo, conf)
			if errors.Is(err, schedulemanager.ErrNoCache) {
				// Do not send notifcation, just update cache
				_, err := sm.services.ScheduleManager.UpdateCache(sm.services.Repo, sm.services.Browser, conf)
				errs = append(errs, err)
				continue
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}

			newRawSchedule, err := sm.services.ScheduleManager.UpdateCache(sm.services.Repo, sm.services.Browser, conf)
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
				updates <- change
			}
		}

		time.Sleep(interval)
	}
}

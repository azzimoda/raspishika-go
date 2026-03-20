package sendings

import (
	"context"
	"encoding/json"
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

func (sm *SendingManager) RunChangeAlertNotifier(ctx context.Context) {
	log.Info().Msg("Change alert notifier started")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var changes = make(chan *models.ScheduleChange)

	go sm.runSchedulepdateMonitor(ctx, changes)

	log.Debug().Msg("Change worker started")
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Change alert notifier stopped")
			return
		case change := <-changes:
			log.Debug().Any("groupName", change.Old.Config.Group.GroupName).Msg("Received schedule change")
			var err error
			elapsed := measureTime(func() {
				err = sm.sendChangeAlertForGroup(ctx, change)
			})
			if err != nil {
				sm.services.Reporter.Report().Log().Err(err).
					Debug("elapsed", elapsed).
					Msg("Errors while change alert sendings")
			}
		default:
			time.Sleep(time.Second)
		}
	}
}

func (sm *SendingManager) sendChangeAlertForGroup(ctx context.Context, change *models.ScheduleChange) error {
	log.Trace().Any("config", change.New.Config).Msg("Sending change alert for group...")
	chats, err := models.GetChatsByMonitoredGroup(sm.services.Repo.DB, change.New.Config.Group.GroupName)
	if err != nil {
		return err
	}

	var errs []error
	for _, chat := range chats {
		if err := sm.sendChangeAlert(ctx, change, &chat); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (sm *SendingManager) sendChangeAlert(
	ctx context.Context,
	change *models.ScheduleChange,
	chat *models.Chat,
) error {
	var errs []error

	conf := change.New.Config
	conf.IsDark = chat.DarkMode
	imageFileName, imageData, err := sm.bot.PrepareScheduleImage(conf)
	if err != nil {
		return fmt.Errorf("failed to prepare schedule image: %w", err)
	}

	text := change.HTML()
	if _, err = sm.bot.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chat.TgChatID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	}); err != nil {
		return fmt.Errorf("failed to send schedule change alert: %w", err)
	}

	if err = sm.bot.SendSchedulePhoto(ctx, sm.bot.Bot, chat, 0, imageFileName, imageData,
		mainbot.WeekScheduleMarkup(conf),
	); err != nil {
		sm.bot.Bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chat.TgChatID,
			Text:   "Не удалось отправить изображение расписания",
		})
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (sm *SendingManager) runSchedulepdateMonitor(ctx context.Context, changes chan<- *models.ScheduleChange) {
	log.Trace().Msg("Schedule update monitor started")

	interval := config.ScheduleUpdateMonitorInterval()
	log.Debug().Dur("interval", interval).Msg("Schedule updating interval")
	for {
		select {
		case <-ctx.Done():
			close(changes)
			log.Debug().Msg("Schedule update monitor stopped")
			return
		default:
		}

		// Fetch groups
		groups, err := models.GetMonitoredGroups(sm.services.Repo.DB)
		if err != nil {
			sm.services.Reporter.Report().Log().Err(err).Msg("Failed to get monitored groups")
			time.Sleep(interval / 2) // Sleep half the interval before retrying
			continue
		}

		chatCount, err := models.GetChatCountWithChangeAlertOn(sm.services.Repo.DB)
		if err != nil {
			sm.services.Reporter.Report().Log().Err(err).Msg("Failed to get chats with update notification enabled")
			chatCount = -1
		}

		log.Debug().Int("groups", len(groups)).Int("chats", chatCount).Msg("Checking schedule updates...")

		changesDetected, chatsAffected, elapsed, errs := sm.fetchScheduleUpdates(groups, changes)

		errSending := errors.Join(errs...)
		errStr := ""
		if errSending != nil {
			errStr = errSending.Error()
		}

		// Log statistics
		if len(groups) > 0 && changesDetected > 0 {
			elapsedPerChat := elapsed / time.Duration(chatCount)
			elapsedPerGroup := elapsed / time.Duration(len(groups))
			log.Debug().Int("groupCount", len(groups)).Dur("elapsed", elapsed).
				Dur("elapsedPerGroup", elapsedPerGroup).
				Msgf("Monitored groups schedules updated in %v (%v/group)", elapsed, elapsedPerGroup)

			if err := models.InsertSendingLog(sm.services.Repo.DB, models.SendingLog{
				Kind:    models.SendingLogChange,
				Chats:   chatsAffected,
				Groups:  changesDetected,
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

			err = errors.Join(errs...)
			if err != nil {
				sm.services.Reporter.Report().Log().Err(err).Debug("groupCount", len(groups)).Debug("elapsed", elapsed).
					Msg("Errors while fetchig updates for monitored groups")
			} else if elapsedPerGroup.Seconds() > 10 {
				sm.services.Reporter.Report().Log().Debug("groupCount", len(groups)).Debug("elapsed", elapsed).
					Msg("Updating monitored groups schedules took too long")
			}
		}
		time.Sleep(interval)
	}
}

func (sm *SendingManager) fetchScheduleUpdates(
	groups []models.Group,
	changes chan<- *models.ScheduleChange,
) (int, int, time.Duration, []error) {
	changesDetected := 0
	chatsAffected := 0
	var errs []error

	elapsed := measureTime(func() {
		for _, group := range groups {
			conf := models.GroupScheduleConfig(&group, false)

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
			diffs := change.Diffs()
			if len(diffs) > 0 {
				log.Debug().Any("config", conf).Msg("Schedule change detected")

				changesDetected++
				if chatsCount, err := models.GetChatCountByGroup(
					sm.services.Repo.DB,
					group.GroupName,
				); err != nil {
					chatsAffected++
				} else {
					chatsAffected += chatsCount
				}

				sm.sendChangeAlertReport(diffs, change)

				// Send the change to channel
				changes <- change
			}
		}
	})

	return changesDetected, chatsAffected, elapsed, errs
}

func (sm *SendingManager) sendChangeAlertReport(diffs []models.Diff, change *models.ScheduleChange) {
	var debugDiffs []models.Diff
	for _, d := range diffs {
		dd := d
		dd.OldDay = nil
		dd.NewDay = nil
		debugDiffs = append(debugDiffs, dd)
	}
	diffsJSON, err := json.MarshalIndent(&debugDiffs, "", "  ")
	diffsJSONStr := string(diffsJSON)
	if err != nil {
		sm.services.Reporter.Report().Err(err).Msg("Failed to marshal schedule change")
		diffsJSONStr = "failed to marshal diffs"
	}

	if _, err = sm.services.Reporter.Report().Log().Debug("diffs", diffsJSONStr).Send(); err != nil {
		sm.services.Reporter.Report().Err(err).Msg("Failed to send change alert report")
	}
	if _, err = sm.services.Reporter.Report().Msgf("Schedule change:\n%s", change.HTML()); err != nil {
		sm.services.Reporter.Report().Err(err).Msg("Failed to send change alert report")
	}
}

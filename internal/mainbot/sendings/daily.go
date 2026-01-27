package sendings

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
)

func (sm *SendingManager) processDailySending(t time.Time) {
	log.Trace().Time("time", t).Msg("Processing daily sending...")
	startTime := time.Now()
	timeStr := t.Format("15:04")

	// Get chats daily sending configured to current time
	chats, err := sm.services.Repo.GetChatsByDailySendingTime(timeStr)
	if err != nil {
		log.Error().Err(err).Time("time", t).Msg("Failed to get chats by daily sending time")
		sm.services.Reporter.Report().Err(err).Msg("Failed to get chats by daily sending time")
		return
	}

	chatsCount := len(chats)
	if chatsCount == 0 {
		log.Trace().Time("time", t).Msg("No chats with daily sending enabled")
		return
	}
	log.Info().Time("time", t).Int("chatCount", chatsCount).Msg("Processing daily sending...")

	// Group chats by configured group
	groupedChats := make(map[string][]*database.Chat)
	for _, chat := range chats {
		if groupedChats[*chat.GroupName] == nil {
			groupedChats[*chat.GroupName] = []*database.Chat{}
		}
		groupedChats[*chat.GroupName] = append(groupedChats[*chat.GroupName], &chat)
	}
	groupsCount := len(groupedChats)
	log.Debug().Time("time", t).Int("groups", groupsCount).Msg("Chats grouped by group name")

	errs, errCount := sm.sendDailyNotificationToGroups(groupedChats)

	// Log statistics
	// TODO: Save daily sending statistics to DB.
	elapsedTime := time.Since(startTime)
	if err := errors.Join(errs...); err != nil {
		sm.services.Reporter.Report().Log().
			Err(err).
			Debug("time", t).
			Debug("elapsed", elapsedTime).
			Debug("chats", chatsCount).
			Debug("groups", groupsCount).
			Msg("Errors while daily sending")
	}

	log.Info().
		Time("time", t).
		Dur("elapsed", elapsedTime).
		Int("chats", chatsCount).
		Int("groups", groupsCount).
		Int("ok", chatsCount-errCount).
		Int("errs", errCount).
		Msgf("Daily sending for time %s finished", timeStr)

	takenTimeFloat := float64(elapsedTime)
	takenTimePerChat := takenTimeFloat / float64(chatsCount)
	if takenTimeFloat > 1.5*float64(time.Minute) || takenTimePerChat > float64(10*time.Second) {
		sm.services.Reporter.Report().Log().
			Debug("time", t).
			Debug("elapsed", elapsedTime).
			Debug("chats", chatsCount).
			Debug("groups", groupsCount).
			Msg("Daily sending took too long")
	}
}

// sendDailyNotificationToGroups sends daily notifications to each chat in each group in parallel.
//
// Returns a slice of errors and the total number of failed chats.
func (sm *SendingManager) sendDailyNotificationToGroups(groupedChats map[string][]*database.Chat) ([]error, int) {
	var wg sync.WaitGroup
	results := make(chan sendingResult, 64)
	workers := viper.GetInt("sending.workers") // TODO: Add this for pair sending.
	if workers == 0 {
		workers = 20 // Default
	}
	semaphore := make(chan struct{}, workers) // Limits the number of concurrent goroutines

	for groupName, chats := range groupedChats {
		wg.Go(func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			groupErrs, failedAll := sm.sendWeekScheduleToGroup(groupName, chats)
			results <- sendingResult{len(chats), groupErrs, failedAll}
		})
	}

	// Wait for all notifications to finish
	go func() {
		log.Trace().Msg("Waiting for daily sending to finish...")
		wg.Wait()
		close(results)
	}()

	// Collect results
	var errs []error
	errCount := 0
	i := 0
	for res := range results {
		if res.failedAll {
			errCount += res.chatsNum
		} else if len(res.errs) != 0 {
			errCount += len(res.errs)
		}
		errs = append(errs, res.errs...)
		i++
	}
	return errs, errCount
}

func (sm *SendingManager) sendWeekScheduleToGroup(groupName string, chats []*database.Chat) ([]error, bool) {
	// Prepare schedule
	log.Trace().Str("group", groupName).Msg("Preparing schedule for group...")
	group, err := sm.services.Repo.GetGroupByName(groupName)
	if err != nil {
		return []error{fmt.Errorf("failed to get group by name %s", groupName)}, true
	}
	scheduleCfg := scraper.GroupScheduleConfig(group)

	ctx := context.Background()
	imageFilename, imageData, err := sm.bot.PrepareWeekScheduleData(
		ctx,
		sm.bot.Bot,
		chats[0].TgChatID,
		0, // NOTE: By default bot sends daily sending to the general chat.
		scheduleCfg,
	)
	if err != nil {
		return []error{fmt.Errorf("failed preparing week schedule data: %w", err)}, true
	}

	// Send schedule to chats
	log.Trace().Str("group", groupName).Msg("Sending daily notification for group...")
	var wg sync.WaitGroup
	results := make(chan error, 64)
	for _, chat := range chats {
		wg.Go(func() {
			// NOTE: By default bot send dailt sending to general chat, so the message thread ID is 0.
			err := sm.bot.SendWeekScheduleMessages(ctx, sm.bot.Bot, 0, chat, scheduleCfg, imageFilename, imageData)
			if err != nil {
				log.Error().Err(err).Int64("chat_id", chat.TgChatID).Msgf("Failed to send daily schedule to chat")
				if err = handleTelegramAPIError(sm.services, chat, err); err == nil {
					return
				}
				results <- err
			}
		})
	}

	go func() {
		log.Trace().Str("group", groupName).Msg("Waiting for results...")
		wg.Wait()
		close(results)
	}()

	var errs []error
	for res := range results {
		errs = append(errs, res)
	}

	log.Trace().Str("group", groupName).Msg("Done sending daily notification for group.")
	return errs, false
}

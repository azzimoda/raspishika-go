package sendings

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
)

func (sm *SendingManager) processDailySending(t time.Time) {
	log.Trace().Time("sendingTime", t).Msg("Processing daily sending...")
	startTime := time.Now()
	timeStr := t.Format("15:04")

	// Get chats daily sending configured to current time
	chats, err := sm.services.Repo.GetChatsByDailySendingTime(timeStr)
	if err != nil {
		log.Error().Err(err).Time("sendingTime", t).Msg("Failed to get chats by daily sending time")
		sm.services.Reporter.Report().Err(err).Msg("Failed to get chats by daily sending time")
		return
	}

	if len(chats) == 0 {
		log.Trace().Time("sendingTime", t).Msg("No chats with daily sending enabled")
		return
	}
	log.Info().Time("sendingTime", t).Int("chatCount", len(chats)).Msg("Processing daily sending...")

	// Group chats by configured group
	groupedChats := make(map[string][]*database.Chat)
	for _, chat := range chats {
		if groupedChats[*chat.GroupName] == nil {
			groupedChats[*chat.GroupName] = []*database.Chat{}
		}
		groupedChats[*chat.GroupName] = append(groupedChats[*chat.GroupName], &chat)
	}
	log.Debug().Time("sendingTime", t).Int("groupCount", len(groupedChats)).Msg("Chats grouped by group name")

	errs, errCount := sm.sendDailyNotificationToGroups(groupedChats)
	if err := errors.Join(errs...); err != nil {
		log.Error().Err(err).Msg("Errors while daily sending")
		sm.services.Reporter.Report().Err(err).Msg("Errors while daily sending")
	}

	// TODO: Save daily sending statistics to DB.

	// Log statistics
	takenTime := time.Since(startTime)
	log.Info().
		Time("sendingTime", t).
		Int("okCount", len(chats)-errCount).
		Int("errCount", errCount).
		Dur("timeTaken", takenTime).
		Msgf("Daily sending for time %s finished", timeStr)

	takenTimeFloat := float64(takenTime)
	takenTimePerChat := takenTimeFloat / float64(len(chats))
	if takenTimeFloat > 1.5*float64(time.Minute) || takenTimePerChat > float64(10*time.Second) {
		sm.services.Reporter.Report().Msgf("Daily sending for time %s took too long (%s)", t, takenTime)
	}
}

// sendDailyNotificationToGroups sends daily notifications to each chat in each group in parallel.
//
// Returns a slice of errors and the total number of failed chats.
func (sm *SendingManager) sendDailyNotificationToGroups(groupedChats map[string][]*database.Chat) ([]error, int) {
	var wg sync.WaitGroup
	results := make(chan sendingResult, 50)
	semaphore := make(chan struct{}, 15) // Limits the number of concurrent goroutines

	for groupName, chats := range groupedChats {
		wg.Add(1)
		go func(groupName string, chats []*database.Chat) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			groupErrs, failedAll := sm.sendWeekScheduleToGroup(groupName, chats)
			results <- sendingResult{len(chats), groupErrs, failedAll}
		}(groupName, chats)
	}

	// Wait for all notifications to finish.
	go func() {
		log.Trace().Msg("Waiting for daily sending to finish...")
		wg.Wait()
		close(results)
	}()

	// Collect results.
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
	log.Trace().Msgf("Sending daily notification to group %s", groupName)

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

	var errs []error
	for _, chat := range chats {
		// NOTE: By default bot send dailt sending to general chat, so the message thread ID is 0.
		err := sm.bot.SendWeekScheduleMessages(ctx, sm.bot.Bot, 0, chat, scheduleCfg, imageFilename, imageData)

		if err != nil {
			log.Error().Err(err).Int64("TgChatID", chat.TgChatID).Msgf("Failed to send daily schedule to chat")
			if err = handleTelegramAPIError(sm.services, chat, err); err == nil {
				continue
			}

			errs = append(errs, err)
		}
	}
	return errs, false
}

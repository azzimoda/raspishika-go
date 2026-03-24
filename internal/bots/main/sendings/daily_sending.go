package sendings

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/models"
)

func (sm *SendingManager) processDailySending(t time.Time) {
	log.Trace().Time("time", t).Msg("Processing daily sending...")
	timeStr := t.Format("15:04")

	// Get chats daily sending configured to current time
	chats, err := models.GetChatsByDailySendingTime(sm.services.Repository.DB, timeStr)
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
	groupedChats := make(map[models.GroupName][]*models.Chat)
	for _, chat := range chats {
		if groupedChats[*chat.GroupName] == nil {
			groupedChats[*chat.GroupName] = []*models.Chat{}
		}
		groupedChats[*chat.GroupName] = append(groupedChats[*chat.GroupName], &chat)
	}
	groupsCount := len(groupedChats)
	log.Debug().Time("time", t).Int("groups", groupsCount).Msg("Chats grouped by group name")

	// Send notifications
	var errs []error
	var errCount int
	elapsedTime := measureTime(func() {
		errs, errCount = sm.sendDailyNotificationToGroups(groupedChats)
	})

	// Log statistics
	elapsedFloat := float64(elapsedTime)
	elapsedPerChat := elapsedFloat / float64(chatsCount)
	elapsedPerGroup := elapsedFloat / float64(groupsCount)
	log.Debug().Dur("elapsedTotal", elapsedTime).Dur("elapsedPerChat", time.Duration(elapsedPerChat)).
		Dur("elapsedPerGroup", time.Duration(elapsedPerGroup)).Send()

	if chatsCount > 0 {
		if err := models.InsertSendingLog(sm.services.Repository.DB, models.SendingLog{
			Kind:    models.SendingLogDaily,
			Chats:   chatsCount,
			Groups:  groupsCount,
			Elapsed: int(elapsedTime.Milliseconds()),
			Fails:   errCount,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to insert daily sending log")
		}

		log.Info().
			Time("time", t).
			Dur("elapsed", elapsedTime).
			Dur("elapsedPerChat", time.Duration(elapsedPerChat)).
			Dur("elapsedPerGroup", time.Duration(elapsedPerGroup)).
			Int("chats", chatsCount).
			Int("groups", groupsCount).
			Int("ok", chatsCount-errCount).
			Int("errs", errCount).
			Msgf("Daily sending for time %s finished", timeStr)
	}

	if err := errors.Join(errs...); err != nil {
		sm.services.Reporter.Report().Log().
			Err(err).
			Debug("time", t).
			Debug("chats", chatsCount).
			Debug("groups", groupsCount).
			Msg("Errors while daily sending")
	}

	if chatsCount > 0 && (elapsedFloat > 1.5*float64(time.Minute) || elapsedPerChat > float64(10*time.Second)) {
		sm.services.Reporter.Report().Log().
			Debug("time", t).
			Debug("elapsed", elapsedTime).
			Debug("elapsedPerChat", elapsedPerChat).
			Debug("elapsedPerGroup", elapsedPerGroup).
			Debug("chats", chatsCount).
			Debug("groups", groupsCount).
			Debug("workers", viper.GetInt(config.KeySendingWorkers)).
			Msg("Daily sending took too long")
	}
}

type sendingResult struct {
	chatsNum  int
	errs      []error
	elapsed   time.Duration
	failedAll bool
}

// sendDailyNotificationToGroups sends daily notifications to each chat in each group in parallel.
//
// Returns a slice of errors and the total number of failed chats.
func (sm *SendingManager) sendDailyNotificationToGroups(groupedChats map[models.GroupName][]*models.Chat) ([]error, int) {
	var wg sync.WaitGroup
	results := make(chan sendingResult, 64)
	workers := viper.GetInt(config.KeySendingWorkers) // TODO: Add this for pair sending.
	if workers == 0 {
		workers = 20 // Default
	}
	log.Trace().Int("workers", workers).Send()
	semaphore := make(chan struct{}, workers) // Limits the number of concurrent goroutines

	for groupName, chats := range groupedChats {
		wg.Go(func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			timeStart := time.Now()
			groupErrs, failedAll := sm.sendWeekScheduleToGroup(groupName, chats)
			results <- sendingResult{len(chats), groupErrs, time.Since(timeStart), failedAll}
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
	totalElapsed := time.Duration(0)
	i := 0
	for res := range results {
		if res.failedAll {
			errCount += res.chatsNum
		} else if len(res.errs) != 0 {
			errCount += len(res.errs)
		}
		errs = append(errs, res.errs...)
		totalElapsed += res.elapsed
		i++
	}
	log.Debug().Dur("elapsedGroupsTotal", totalElapsed).Send()
	return errs, errCount
}

func (sm *SendingManager) sendWeekScheduleToGroup(groupName models.GroupName, chats []*models.Chat) ([]error, bool) {
	log.Trace().Any("group", groupName).Msg("Preparing schedule for group...")
	ctx := context.Background()
	var errors []error

	// Prepare schedule
	group, err := models.GetGroupByName(sm.services.Repository.DB, groupName)
	if err != nil {
		errors = []error{fmt.Errorf("failed to get group by name %s", groupName)}
		return errors, true
	}
	confLight := models.GroupScheduleConfig(group, false)
	confDark := models.GroupScheduleConfig(group, true)
	var imageFilenameLight string
	var imageDataLight []byte
	var imageFilenameDark string
	var imageDataDark []byte
	var doReturn = false
	elapsedFetchGroupSchedule := measureTime(func() {
		_, err = sm.bot.Bot.SendChatAction(ctx, &bot.SendChatActionParams{
			ChatID:          chats[0].TgChatID,
			MessageThreadID: 0,
			Action:          tgmodels.ChatActionTyping,
		})
		imageFilenameLight, imageDataLight, err = sm.bot.PrepareScheduleImage(confLight)
		imageFilenameDark, imageDataDark, err = sm.bot.PrepareScheduleImage(confDark)
		if err != nil {
			errors = []error{fmt.Errorf("failed preparing week schedule data: %w", err)}
			doReturn = true
			return
		}
	})
	log.Trace().Dur("elapsedFetchGroupSchedule", elapsedFetchGroupSchedule).Any("group", groupName).Send()
	if doReturn {
		return errors, true
	}

	var errs []error
	elapsedSendToGroup := measureTime(func() {
		// Send schedule to chats
		log.Trace().Any("group", groupName).Msg("Sending daily notification for group...")
		var wg sync.WaitGroup
		results := make(chan error, 64)
		for _, chat := range chats {
			wg.Go(func() {
				elapsed := measureTime(func() {
					// NOTE: By default bot send dailt sending to general chat, so the message thread ID is 0.
					conf := confLight
					imageFilename := imageFilenameLight
					imageData := imageDataLight
					if chat.DarkMode {
						conf = confDark
						imageFilename = imageFilenameDark
						imageData = imageDataDark
					}

					err := sm.bot.SendWeekScheduleMessages(ctx, sm.bot.Bot, 0, chat, conf, imageFilename, imageData)
					if err != nil {
						log.Error().Err(err).Any("chat_id", chat.TgChatID).
							Msgf("Failed to send daily schedule to chat")
						if err = handleTelegramAPIError(sm.services, chat, err); err == nil {
							return
						}
						results <- err
					}
				})
				log.Trace().Dur("elapsedChat", elapsed).Any("group", groupName).Any("chat", chat).Send()
			})
		}

		go func() {
			log.Trace().Any("group", groupName).Msg("Waiting for results...")
			wg.Wait()
			close(results)
		}()

		var errs []error
		for res := range results {
			errs = append(errs, res)
		}
	})
	log.Trace().Dur("elapsendSendToGroup", elapsedSendToGroup).Any("group", groupName).Send()

	log.Trace().Any("group", groupName).Msg("Done sending daily notification for group.")
	return errs, false
}

func measureTime(f func()) time.Duration {
	start := time.Now()
	f()
	return time.Since(start)
}

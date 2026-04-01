package broadcast

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/bot/botutil"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
)

func (s *BroadcastService) processDaily(t time.Time) {
	if s.Bot == nil {
		log.Warn().Msg("Bot is not set yet")
		return
	}

	log.Trace().Time("time", t).Msg("Processing daily broadcast...")
	timeStr := t.Format("15:04")

	chats, err := s.Chat.AllByDailyTime(timeStr)
	if err != nil {
		s.Report().Log().Err(err).Debug("time", timeStr).Msg("Failed to get chats by daily broadcat time")
		return
	}

	chatCount := len(chats)
	if chatCount == 0 {
		log.Trace().Time("time", t).Msg("No chats with daily broadcast enabled")
		return
	}
	log.Debug().Time("time", t).Int("chats", chatCount).Msg("Processing daily broadcast...")

	grouped := groupChats(chats)
	groupCount := len(grouped)
	log.Debug().Time("time", t).Int("groups", groupCount).Int("chats", chatCount).Send()

	// Fetch and send schedules
	var errs []error
	var errCount int
	elapsed := measureTime(func() { errs, errCount = s.sendDaily(grouped) })

	// Log stats
	s.log(t, elapsed, chatCount, groupCount, errCount, errs)
}

type sendResult struct {
	chats     int
	errs      []error
	elapsed   time.Duration
	failedAll bool
}

func (s *BroadcastService) sendDaily(grouped GroupedChats) ([]error, int) {
	var wg sync.WaitGroup

	results := make(chan sendResult)
	workers := viper.GetInt(config.KeySendingWorkers)
	if workers < 1 {
		workers = 20 // Fallback
	}
	log.Trace().Int("workers", workers).Send()
	semaphore := make(chan struct{}, workers)

	for groupName, chats := range grouped {
		wg.Go(func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			timeStart := time.Now()
			groupErrs, failedAll := s.sendDailyToGroup(groupName, chats)
			results <- sendResult{
				chats:     len(chats),
				errs:      groupErrs,
				elapsed:   time.Since(timeStart),
				failedAll: failedAll,
			}
		})
	}

	go func() {
		log.Trace().Msg("Waiting for results...")
		wg.Wait()
		close(results)
	}()

	var errs []error
	errCount := 0
	i := 0
	for res := range results {
		if res.failedAll {
			errCount += res.chats
		}
		errCount += len(res.errs)
		errs = append(errs, res.errs...)
		i++
	}
	return errs, errCount
}

func (s *BroadcastService) sendDailyToGroup(groupName model.GroupName, chats []*model.Chat) ([]error, bool) {
	var errs []error

	group, err := s.Group.GetByName(groupName)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to get group by name %s", groupName))
		return errs, true
	}

	// Send chat action
	go func() {
		ctx := context.Background()
		for _, chat := range chats {
			s.SendChatAction(ctx, &bot.SendChatActionParams{
				ChatID:          chat.TgChatID,
				MessageThreadID: 0,
				Action:          tgmodels.ChatActionTyping,
			})
		}
	}()

	// Fetch schedule
	confLight := model.GroupScheduleConfig(group, false)
	confDark := model.GroupScheduleConfig(group, true)
	imageFilenameLight, imageDataLight, err := s.scheduleService.PrepareScheduleImage(confLight)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to prepare week schedule image: %w", err))
		return errs, true
	}
	imageFilenameDark, imageDataDark, err := s.scheduleService.PrepareScheduleImage(confDark)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to prepare week schedule image: %w", err))
		return errs, true
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 64)
	for _, chat := range chats {
		wg.Go(func() {
			conf := confLight
			imageFilename := imageFilenameLight
			imageData := imageDataLight
			if chat.DarkMode {
				conf = confDark
				imageFilename = imageFilenameDark
				imageData = imageDataDark
			}

			err := botutil.SendWeekScheduleMessages(ctx, s.Bot, 0, chat, conf, imageFilename, imageData)
			if err != nil {
				// log.Error().Err(err).Any("chatID", chat.TgChatID).Msg("Failed to send daily message to chat")

				if err := s.handleAPIError(ctx, chat, err); err == nil {
					return
				}
				results <- err
			}
		})
	}

	go func() {
		log.Trace().Str("group", groupName.String()).Msg("Waiting for results")
		wg.Wait()
		close(results)
	}()

	for err := range results {
		errs = append(errs, err)
	}

	return errs, false
}

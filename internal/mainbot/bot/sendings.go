package bot

import (
	"errors"
	"fmt"
	"time"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

func (b *Bot) ScheduleDailySending(c *cron.Cron) error {
	_, err := c.AddFunc("* * * * *", func() { go b.processDailySending() })
	return err
}

func (b *Bot) processDailySending() {
	timeStart := time.Now()
	timeStr := time.Now().Format("15:04")

	chats, err := b.repo.GetChatsByDailySendingTime(timeStr)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats by daily sending time")
		b.Report().Err(err).Send("Failed to get chats by daily sending time")
		return
	}

	if len(chats) == 0 {
		return
	}
	log.Debug().Msgf("Processing daily sending for time %s", timeStr)

	groupedChats := make(map[string][]*database.Chat)
	for _, chat := range chats {
		if groupedChats[*chat.GroupName] == nil {
			groupedChats[*chat.GroupName] = []*database.Chat{}
		}

		groupedChats[*chat.GroupName] = append(groupedChats[*chat.GroupName], &chat)
	}

	okCount := 0
	errCount := 0
	for groupName, chats := range groupedChats {
		log.Trace().Msgf("Sending daily notification to group %s", groupName)
		group, err := b.repo.GetGroupByName(groupName)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to get group by name %s", groupName)
			b.Report().Err(err).Send(fmt.Sprintf("Failed to get group by name %s", groupName))
			continue
		}
		scheduleCfg := scraper.GroupScheduleConfig(group)

		for _, chat := range chats {
			if err := b.CommandHandler.SendWeekSchedule(chat, scheduleCfg); err != nil {
				log.Error().Err(err).Int64("tgChatID", chat.TgChatID).Msgf("Failed to send daily schedule to chat")
				b.Report().Chat(chat).Err(err).Send("Failed to send daily schedule to chat")
				errCount++
			} else {
				okCount++
			}
		}
	}

	log.Debug().Int("okCount", okCount).Int("errCount", errCount).Dur("timeTaken", time.Since(timeStart)).
		Msgf("Daily sending for time %s finished", timeStr)
	// TODO: Implement reporting on too long processing time.
}

func (b *Bot) SchedulePairSending(c *cron.Cron) error {
	times := [][2]int{
		{7, 45},  // 8:00
		{9, 30},  // 9:45
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
		_, err := c.AddFunc(fmt.Sprintf("%d %d * * 1-6", m, h), func() { go b.processPairSending(time.Now()) })
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) processPairSending(startTime time.Time) {
	pairTime := startTime.Add(15 * time.Minute)
	timeStr := pairTime.Format("15:04")
	log.Trace().Msgf("Processing pair sending for time %s", timeStr)

	chats, err := b.repo.GetChatsWithPairSendingEnabled()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with pair sending enabled")
		b.Report().Err(err).Send("Failed to get chats with pair sending enabled")
		return
	}

	if len(chats) == 0 {
		log.Trace().Msg("No chats with pair sending enabled")
		return
	}
	log.Debug().Msgf("Processing daily sending for time %s to %d chats", timeStr, len(chats))

	groupedChats := make(map[string][]int64)
	for _, chat := range chats {
		if groupedChats[*chat.GroupName] == nil {
			groupedChats[*chat.GroupName] = []int64{}
		}

		groupedChats[*chat.GroupName] = append(groupedChats[*chat.GroupName], chat.TgChatID)
	}

	okCount := 0
	errCount := 0
	for groupName, chatIDs := range groupedChats {
		err = b.sendPairNotificationToGroup(groupName, pairTime, chatIDs)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to send pair notification to group %s", groupName)
			log.Error().Err(err).Msg(errMsg)
			b.Report().Err(err).Send(fmt.Sprint(errMsg))
			errCount++
		} else {
			okCount++
		}
	}

	log.Debug().Int("okCount", okCount).Int("errCount", errCount).Dur("timeTaken", time.Since(startTime)).
		Msgf("Pair sending for time %s finished", timeStr)
	// TODO: Implement reporting on too long processing time.
}

func (b *Bot) sendPairNotificationToGroup(groupName string, pairTime time.Time, tgChatIDs []int64) error {
	log.Trace().Msgf("Sending pair notification to group %s (%d chats)", groupName, len(tgChatIDs))

	group, err := b.repo.GetGroupByName(groupName)
	if err != nil {
		return fmt.Errorf("failed to get group by name %s: %w", groupName, err)
	}

	rawSchedule, err := scraper.FetchSchedule(b.repo, b.config.Cache.Dir, scraper.GroupScheduleConfig(group))
	if err != nil {
		return fmt.Errorf("failed to fetch schedule for group %s: %w", groupName, err)
	}

	firstPairTime := time.Date(pairTime.Year(), pairTime.Month(), pairTime.Day(), 8, 0, 0, 0, pairTime.Location())
	scheduleDay := rawSchedule.Transform().Days[0]
	log.Trace().Msgf("Current day: %s", scheduleDay.Date)
	pair, err := scheduleDay.CurrentPair(pairTime)
	if err != nil {
		if pairTime.Before(firstPairTime) {
			pair = &scheduleDay.Pairs[0]
		}
		log.Trace().Err(err).Msg("There is no pair in 15 minutes")
		return nil
	}

	text := ""
	switch pair.Kind {
	case scraper.PairKindEmpty, scraper.PairKindEvent, scraper.PairKindIGA, scraper.PairKindVacation,
		scraper.PairKindPractice:

		log.Trace().Str("kind", string(pair.Kind)).Msg("Pair is empty")
		return nil
	default:
		text = fmt.Sprintf("Следующая пара в кабинете %s:\n    *%s*\n    %s",
			tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, pair.Classroom),
			tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, pair.Discipline),
			tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, *pair.Teacher))
		// TODO: Use tgbotapi.EscapeText() instead of my own implementation.
	}

	errs := make([]error, 0)
	for _, tgChatID := range tgChatIDs {
		msg := tgbotapi.NewMessage(tgChatID, text)
		msg.ParseMode = tgbotapi.ModeMarkdownV2
		if _, err := b.api.Send(msg); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

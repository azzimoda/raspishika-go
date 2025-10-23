package bot

import (
	"time"

	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"

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

	chats, err := b.Repo.GetChatsByDailySendingTime(timeStr)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats by daily sending time")
		b.Reporter.Report().Err(err).Send("Failed to get chats by daily sending time")
		return
	}

	if len(chats) == 0 {
		return
	}
	log.Debug().Msgf("Processing daily sending for time %s", timeStr)

	groupedChats := make(map[string][]int64)
	for _, chat := range chats {
		if groupedChats[*chat.GroupName] == nil {
			groupedChats[*chat.GroupName] = []int64{}
		}

		groupedChats[*chat.GroupName] = append(groupedChats[*chat.GroupName], chat.ChatID)
	}

	okCount := 0
	errCount := 0
	for groupName, chatIDs := range groupedChats {
		log.Trace().Msgf("Sending daily sending to group %s", groupName)

		for _, chatID := range chatIDs {
			if err := commands.SendWeekSchedule(
				b.api, b.Repo, b.Browser, b.Cache, b.Config.Browser.ScreenshotDir, b.Config.ScheduleTemplate, chatID,
				groupName,
			); err != nil {
				log.Error().Err(err).Int64("chatID", chatID).Msgf("Failed to send daily schedule to chat")
				b.Reporter.Report().Chat(chatID).Err(err).Send("Failed to send daily schedule to chat")
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

// TODO: func (b *Bot) SchedulePairSending(c *cron.Cron) error {}

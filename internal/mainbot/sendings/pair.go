package sendings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (sm *SendingManager) processPairSending(t time.Time) {
	pairTime := t.Add(15 * time.Minute)
	timeStr := pairTime.Format("15:04")
	log.Trace().Msgf("Processing pair sending for time %s", timeStr)

	chats, err := sm.services.Repo.GetChatsWithPairSendingEnabled()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with pair sending enabled")
		sm.services.Reporter.Report().Err(err).Msg("Failed to get chats with pair sending enabled")
		return
	}

	if len(chats) == 0 {
		log.Trace().Msg("No chats with pair sending enabled")
		return
	}
	log.Debug().Msgf("Processing pair sending for time %s to %d chats", timeStr, len(chats))

	groupedChats := make(map[string][]*database.Chat)
	for _, chat := range chats {
		if groupedChats[*chat.GroupName] == nil {
			groupedChats[*chat.GroupName] = []*database.Chat{}
		}

		groupedChats[*chat.GroupName] = append(groupedChats[*chat.GroupName], &chat)
	}

	var errs []error
	errCount := 0
	for groupName, chats := range groupedChats {
		groupErrs, failedAll := sm.sendPairNotificationToGroup(groupName, pairTime, chats)
		if failedAll {
			errCount += len(chats)
		} else if len(groupErrs) != 0 {
			errCount += len(groupErrs)
		}
		errs = append(errs, groupErrs...)
	}

	if len(errs) != 0 {
		err := errors.Join(errs...)
		log.Error().Err(err).Msg("Errors while sending pair notification")
		sm.services.Reporter.Report().Err(err).Msg("Errors while sending pair notification")
	}

	takenTime := time.Since(t)
	log.Info().
		Int("okCount", len(chats)-errCount).
		Int("errCount", errCount).
		Dur("timeTaken", takenTime).
		Msgf("Pair sending for time %s finished", timeStr)

	takenTimeFloat := float64(takenTime)
	takenTimePerChat := takenTimeFloat / float64(len(chats))
	if takenTimeFloat > 1.5*float64(time.Minute) || takenTimePerChat > float64(10*time.Second) {
		sm.services.Reporter.Report().Msgf("Daily sending for time %s took too long (%s)", t, takenTime)
	}
}

func (sm *SendingManager) sendPairNotificationToGroup(
	groupName string,
	pairTime time.Time,
	chats []*database.Chat,
) ([]error, bool) {
	log.Trace().Msgf("Sending pair notification to group %s (%d chats)", groupName, len(chats))

	group, err := sm.bot.FetchGroupByNameWithValidation(groupName)
	if errors.Is(err, mainbot.ErrGroupNotFound) || errors.Is(err, mainbot.ErrWrongGroupNameFormat) {
		log.Error().Err(err).Str("groupName", groupName).Msg("Failed to get group by name")

		// Clear users' configured group.
		for _, chat := range chats {
			chat.GroupName = nil
			chat.DepartmentName = nil
			if err := sm.services.Repo.UpdateChat(chat); err != nil {
				log.Error().Err(err).Int64("tgChatID", chat.TgChatID).Msg("Failed to update chat")
			} else {
				tgbothelpers.SendTempMessage(context.Background(), sm.bot.Bot, 5*time.Minute, &bot.SendMessageParams{
					ChatID: chat.TgChatID,
					Text:   "Не удалось получить расписание группы. Задайте группу заново: /group",
				})
			}
		}

		return []error{fmt.Errorf("failed to get group by name %s: %w", groupName, err)}, true
	}

	rawSchedule, err := sm.services.ScheduleManager.Get(
		sm.services.Repo, sm.services.Browser, sm.services.Cache, scraper.GroupScheduleConfig(group))
	if err != nil {
		return []error{fmt.Errorf("failed to fetch schedule for group %s: %w", groupName, err)}, true
	}

	scheduleDay := rawSchedule.Transform().Days[0]
	log.Trace().Msgf("Current day: %s", scheduleDay.Date)
	pair, err := scheduleDay.CurrentPair(pairTime)
	if err != nil {
		log.Trace().Err(err).Msg("There is no pair in 15 minutes")
		return nil, false
	}

	text := ""
	switch pair.Kind {
	case scraper.PairKindEmpty, scraper.PairKindEvent, scraper.PairKindIGA, scraper.PairKindVacation,
		scraper.PairKindPractice:

		log.Trace().Str("kind", string(pair.Kind)).Msg("Pair is empty")
		return nil, false
	default:
		text = fmt.Sprintf("Следующая пара в кабинете %s:\n\t*%s*\n\t%s",
			bot.EscapeMarkdown(pair.Classroom),
			bot.EscapeMarkdown(pair.Discipline),
			bot.EscapeMarkdown(utils.DerefOrTypeDefault(pair.Teacher)))
	}

	messagesToDelete := make([]*models.Message, 0)
	errs := make([]error, 0)
	for _, chat := range chats {
		if msg, err := sm.bot.Bot.SendMessage(context.Background(), &bot.SendMessageParams{
			ChatID:    chat.TgChatID,
			Text:      text,
			ParseMode: models.ParseModeMarkdown,
		}); err != nil {
			if err = handleTelegramAPIError(sm.services, chat, err); err == nil {
				continue
			}
			errs = append(errs, err)
		} else {
			messagesToDelete = append(messagesToDelete, msg)
			log.Trace().Int64("chatID", chat.TgChatID).Msg("Pair notification sent")
		}
	}

	// Delete notifications after a while.
	if len(messagesToDelete) != 0 {
		go func() {
			log.Trace().Dur("PairNotificationTTLDuration", sm.config.Sendings.PairNotificationTTLDuration()).Send()
			time.Sleep(sm.config.Sendings.PairNotificationTTLDuration())
			for _, msg := range messagesToDelete {
				_, err := tgbothelpers.DeleteMessageSafely(context.Background(), sm.bot.Bot, msg)
				if err != nil {
					log.Error().Err(err).Msg("Failed to delete pair notification message")
				}
			}
		}()
	}

	return errs, false
}

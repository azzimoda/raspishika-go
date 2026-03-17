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
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (sm *SendingManager) processPairSending(t time.Time) {
	pairTime := t.Add(15 * time.Minute)
	timeStr := pairTime.Format("15:04")
	log.Trace().Msgf("Processing pair sending for time %s", timeStr)

	chats, err := models.GetChatsWithPairSendingEnabled(sm.services.Repo.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with pair sending enabled")
		sm.services.Reporter.Report().Err(err).Msg("Failed to get chats with pair sending enabled")
		return
	}

	chatCount := len(chats)
	if chatCount == 0 {
		log.Trace().Msg("No chats with pair sending enabled")
		return
	}
	log.Debug().Msgf("Processing pair sending for time %s to %d chats", timeStr, chatCount)

	groupedChats := make(map[models.GroupName][]*models.Chat)
	for _, chat := range chats {
		if groupedChats[*chat.GroupName] == nil {
			groupedChats[*chat.GroupName] = []*models.Chat{}
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

	// Log stats
	elapsed := time.Since(t)

	groupCount := len(groupedChats)
	if chatCount > 0 {
		if err := models.InsertSendingLog(sm.services.Repo.DB, models.SendingLog{
			Kind:    models.SendingLogPair,
			Chats:   chatCount,
			Groups:  groupCount,
			Elapsed: int(elapsed.Milliseconds()),
			Fails:   errCount,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to insert pair notification log")
		}
	}

	elapsedFloat := float64(elapsed)
	elapsedPerChat := elapsedFloat / float64(chatCount)
	elapsedPerGroup := elapsedFloat / float64(groupCount)
	if elapsedPerGroup > float64(10*time.Second) {
		sm.services.Reporter.Report().Log().
			Debug("time", t).
			Debug("elapsed", elapsed).
			Debug("elapsedPerChat", time.Duration(elapsedPerChat)).
			Debug("elapsedPerGroup", time.Duration(elapsedPerGroup)).
			Debug("chats", chatCount).
			Debug("groups", groupCount).
			Debug("ok", chatCount-errCount).
			Debug("err", errCount).
			Msgf("Daily sending for time %s took too long (%s, %s/group)",
				t, elapsed, time.Duration(elapsedPerGroup))
	} else {
		log.Info().
			Time("time", t).
			Dur("elapsed", elapsed).
			Dur("elapsedPerChat", time.Duration(elapsedPerChat)).
			Int("chats", chatCount).
			Int("groups", groupCount).
			Int("ok", chatCount-errCount).
			Int("err", errCount).
			Msgf("Pair sending for time %s finished", timeStr)
	}

	if len(errs) != 0 {
		err := errors.Join(errs...)
		sm.services.Reporter.Report().Log().Err(err).Msg("Errors while sending pair notification")
	}
}

func (sm *SendingManager) sendPairNotificationToGroup(
	groupName models.GroupName,
	pairTime time.Time,
	chats []*models.Chat,
) ([]error, bool) {
	log.Trace().Msgf("Sending pair notification to group %s (%d chats)", groupName, len(chats))

	group, err := sm.bot.FetchGroupByNameWithValidation(groupName)
	if errors.Is(err, mainbot.ErrGroupNotFound) || errors.Is(err, mainbot.ErrWrongGroupNameFormat) {
		log.Error().Err(err).Any("groupName", groupName).Msg("Failed to get group by name")

		// Clear users' configured group.
		for _, chat := range chats {
			chat.GroupName = nil
			chat.DepartmentName = nil
			if err := chat.Update(sm.services.Repo.DB); err != nil {
				log.Error().Err(err).Any("tgChatID", chat.TgChatID).Msg("Failed to update chat")
			} else {
				bothelpers.SendTempMessage(context.Background(), sm.bot.Bot, 5*time.Minute, &bot.SendMessageParams{
					ChatID: chat.TgChatID,
					Text:   "Не удалось получить расписание группы. Задайте группу заново: /group",
				})
			}
		}

		return []error{fmt.Errorf("failed to get group by name %s: %w", groupName, err)}, true
	}

	rawSchedule, err :=
		sm.services.ScheduleMan.Get(sm.services.Repo, sm.services.Browser, models.GroupScheduleConfig(group))
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
	case models.PairKindEmpty, models.PairKindEvent, models.PairKindIGA, models.PairKindVacation,
		models.PairKindPractice:

		log.Trace().Str("kind", string(pair.Kind)).Msg("Pair is empty")
		return nil, false
	default:
		text = fmt.Sprintf("Следующая пара в кабинете %s:\n\t<b>%s</b>\n\t%s",
			pair.Classroom, pair.Discipline, utils.DerefOrTypeDefault(pair.Teacher))
	}

	messagesToDelete := make([]*tgmodels.Message, 0)
	errs := make([]error, 0)
	for _, chat := range chats {
		if msg, err := sm.bot.Bot.SendMessage(context.Background(), &bot.SendMessageParams{
			ChatID:    chat.TgChatID,
			Text:      text,
			ParseMode: tgmodels.ParseModeHTML,
		}); err != nil {
			if err = handleTelegramAPIError(sm.services, chat, err); err == nil {
				continue
			}
			errs = append(errs, err)
		} else {
			messagesToDelete = append(messagesToDelete, msg)
			log.Trace().Any("chatID", chat.TgChatID).Msg("Pair notification sent")
		}
	}

	// Delete notifications after a while.
	if len(messagesToDelete) != 0 {
		go func() {
			dur := config.PairNotificationTTLDur() // sm.config.Sendings.PairNotificationTTLDuration()
			log.Trace().Dur("PairNotificationTTLDuration", dur).Send()
			time.Sleep(dur)
			for _, msg := range messagesToDelete {
				_, err := bothelpers.DeleteMessageSafely(context.Background(), sm.bot.Bot, msg)
				if err != nil {
					log.Error().Err(err).Msg("Failed to delete pair notification message")
				}
			}
		}()
	}

	return errs, false
}

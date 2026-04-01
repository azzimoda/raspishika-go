package broadcast

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/service/scraper"
	"github.com/azzimoda/raspishika-go/pkg/bothelper"
	"github.com/azzimoda/raspishika-go/pkg/refutil"
)

func (s *BroadcastService) processPairNotification(t time.Time) {
	if s.Bot == nil {
		log.Warn().Msg("Bot is not set yet")
		return
	}

	pairTime := t.Add(15 * time.Minute)
	timeStr := pairTime.Format("15:04")
	log.Trace().Str("pairStart", timeStr).Time("time", t).Msg("Processing pair notification...")

	chats, err := s.Chat.AllWithPairNotification()
	if err != nil {
		s.Report().Log().Err(err).Debug("time", t).Debug("pairStart", timeStr).Msg("Failed to get chats with pair notification enabled")
		return
	}

	chatCount := len(chats)
	if chatCount == 0 {
		log.Trace().Msg("No chats with pair notification enabled")
		return
	}
	log.Debug().Str("pairStart", timeStr).Time("time", t).Int("chats", chatCount).Msg("Processing pair sending...")

	grouped := groupChats(chats)
	groupCount := len(grouped)
	log.Debug().Time("time", t).Int("groups", groupCount).Int("chats", chatCount).Send()

	var errs []error
	var errCount int
	elapsed := measureTime(func() { errs, errCount = s.sendPairNotification(grouped, pairTime) })

	s.log(t, elapsed, chatCount, groupCount, errCount, errs)
}

func (s *BroadcastService) sendPairNotification(grouped GroupedChats, pairTime time.Time) ([]error, int) {
	var errs []error
	errCount := 0
	for groupName, chats := range grouped {
		groupErrs, failedAll := s.sendPairNotificationToGroup(groupName, pairTime, chats)
		if failedAll {
			errCount += len(chats)
		} else {
			errCount += len(groupErrs)
		}
		errs = append(errs, groupErrs...)
	}
	return errs, errCount
}

func (s *BroadcastService) sendPairNotificationToGroup(
	groupName model.GroupName,
	pairTime time.Time,
	chats []*model.Chat,
) ([]error, bool) {
	group, err := scraper.FetchGroupByNameWithValidation(s.Group, s.browser, groupName)
	if errors.Is(err, scraper.ErrGroupNotFound) || errors.Is(err, scraper.ErrWrongGroupNameFormat) {
		log.Error().Err(err).Any("group", groupName).Msg("Failed to get group by name")

		for _, chat := range chats {
			chat.GroupName = nil
			chat.DepartmentName = nil
			if err := s.Chat.Update(chat); err != nil {
				log.Error().Err(err).Int64("chat_id", chat.TgChatID.Int64()).Msg("Failed to update chat")
			} else {
				bothelper.SendTempMessage(context.Background(), s.Bot, 5*time.Minute, &bot.SendMessageParams{
					ChatID: chat.TgChatID,
					Text:   "Не удалось получить расписание группы. Задайте группу заново: /group",
				})
			}
		}

		return []error{fmt.Errorf("failed to get group by name %s: %w", groupName, err)}, true
	}

	rawSchedule, err := s.scheduleService.Get(model.GroupScheduleConfig(group, false))
	if err != nil {
		return []error{fmt.Errorf("faield to fetch schedule for group %s: %w", groupName, err)}, true
	}

	scheduleDay := rawSchedule.Transform().Days[0]
	pair, err := scheduleDay.CurrentPair(pairTime)
	if err != nil {
		return nil, false
	}

	text := ""
	switch pair.Kind {
	case model.PairKindEmpty, model.PairKindEvent, model.PairKindIGA, model.PairKindVacation,
		model.PairKindPractice:

		log.Trace().Str("kind", string(pair.Kind)).Msg("Pair is empty")
		return nil, false
	default:
		text = fmt.Sprintf("Следующая пара в кабинете %s:\n\t<b>%s</b>\n\t%s",
			pair.Classroom, pair.Discipline, refutil.DerefOrTypeDefault(pair.Teacher))
	}

	ctx := context.Background()
	messagesToDelete := make([]*tgmodels.Message, 0)
	errs := make([]error, 0)
	for _, chat := range chats {
		if msg, err := s.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    chat.TgChatID,
			Text:      text,
			ParseMode: tgmodels.ParseModeHTML,
		}); err != nil {
			if err := s.handleAPIError(ctx, chat, err); err != nil {
				continue
			}
			errs = append(errs, err)
		} else {
			messagesToDelete = append(messagesToDelete, msg)
		}
	}

	if len(messagesToDelete) > 0 {
		go func() {
			dur := config.PairNotificationTTLDur()
			time.Sleep(dur)
			for _, msg := range messagesToDelete {
				if _, err := bothelper.DeleteMessageSafely(ctx, s.Bot, msg); err != nil {
					log.Error().Err(err).Msg("Failed to delete pair notification message")
				}
			}
		}()
	}

	return errs, false
}

package broadcast

import (
	"errors"
	"time"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func measureTime(f func()) time.Duration {
	start := time.Now()
	f()
	return time.Since(start)
}

type GroupedChats = map[model.GroupName][]*model.Chat

func groupChats(chats []model.Chat) GroupedChats {
	grouped := make(map[model.GroupName][]*model.Chat)
	for _, chat := range chats {
		grouped[*chat.GroupName] = append(grouped[*chat.GroupName], &chat)
	}
	return grouped
}

func (s *BroadcastService) log(
	t time.Time,
	elapsed time.Duration,
	chatCount int,
	groupCount int,
	errCount int,
	errs []error,
	kind model.SendingLogKind,
) {
	elapsedPerChat := elapsed / time.Duration(chatCount)
	elapsedPerGroup := elapsed / time.Duration(groupCount)

	if chatCount > 0 {
		if err := s.Log.LogSending(model.SendingLog{
			Kind:    model.SendingLogDaily,
			Chats:   chatCount,
			Groups:  groupCount,
			Elapsed: int(elapsed.Milliseconds()),
			Fails:   errCount,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to log broadcast stats")
		}

		log.Info().
			Any("kind", kind).
			Time("time", t).
			Dur("elapsed", elapsed).
			Dur("elapsedPerChat", elapsedPerChat).
			Dur("elapsedPerGroup", elapsedPerGroup).
			Int("chats", chatCount).
			Int("groups", groupCount).
			Int("fails", errCount).
			Msgf("Broadcast finished")
	}

	if err := errors.Join(errs...); err != nil {
		s.Report().Log().Err(err).
			Err(err).
			Debug("kind", string(kind)).
			Debug("time", t).
			Debug("chats", chatCount).
			Debug("groups", groupCount).
			Msg("Errors while broadcasting")
	}

	if chatCount > 0 && (elapsedPerGroup > 10*time.Second) {
		s.Report().Log().
			Debug("kind", string(kind)).
			Debug("time", t).
			Debug("elapsed", elapsed).
			Debug("elapsedPerChat", elapsedPerChat).
			Debug("elapsedPerGroup", elapsedPerGroup).
			Debug("chats", chatCount).
			Debug("groups", groupCount).
			Debug("workers", viper.GetInt(config.KeySendingWorkers)).
			Msg("Broadcast took too long")
	}
}

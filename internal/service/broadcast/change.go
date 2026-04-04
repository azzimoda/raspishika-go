package broadcast

import (
	"context"
	"errors"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/botutil"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/service/schedule"
)

func (s *BroadcastService) processChangeAlert(ctx context.Context) {
	t := time.Now()
	_ = t

	chats, err := s.Chat.AllWithChangeAlert()
	if err != nil {
		s.Report().Log().Err(err).Msg("Failed to get chats with change alert on")
		return
	}
	chatCount := len(chats)
	if chatCount == 0 {
		log.Trace().Msg("No chats with change alert enabled")
		return
	}

	grouped := groupChats(chats)
	groups := make([]*model.Group, 0, len(grouped))
	for gn := range grouped {
		g, err := s.Group.GetByName(gn)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get group by name")
			continue
		}
		groups = append(groups, g)
	}

	groupCount := len(groups)
	log.Debug().Int("groups", groupCount).Int("chats", chatCount).Msg("Checking schedule updates...")

	timeStart := time.Now()
	changes, errsFetch := s.fetchChanges(groups)

	var errsSend []error
	for groupName, change := range changes {
		errsGroup := s.sendChangeAlertForGroup(ctx, change, grouped[groupName])
		errsSend = append(errsSend, errsGroup...)
	}

	errs := append(errsFetch, errsSend...)
	s.log(timeStart, time.Since(timeStart), chatCount, groupCount, len(errsSend), errs, model.SendingLogChange)
}

func (s *BroadcastService) fetchChanges(groups []*model.Group) (map[model.GroupName]*model.ScheduleChange, []error) {
	changesByGroup := make(map[model.GroupName]*model.ScheduleChange)
	var errs []error

	for _, group := range groups {
		conf := model.GroupScheduleConfig(group, false)

		oldRawSchedule, err := s.scheduleService.GetCache(conf)
		if errors.Is(err, schedule.ErrNoCache) {
			log.Warn().Err(err).Any("config", conf).Msg("No cache for the schedule config; just updating cache...")
			if _, err := s.scheduleService.UpdateCache(s.browser, conf); err != nil {
				errs = append(errs, err)
			}
			continue
		} else if err != nil {
			log.Error().Err(err).Any("config", conf).Msg("Failed to get schedule cache")
			errs = append(errs, err)
			continue
		}

		newRawSchedule, err := s.scheduleService.UpdateCache(s.browser, conf)
		if err != nil {
			log.Error().Err(err).Any("config", conf).Msg("Failed to update schedule cache")
			errs = append(errs, err)
			continue
		}

		change := model.NewScheduleChange(oldRawSchedule.Transform(), newRawSchedule.Transform())
		diffs := change.Diffs()
		if len(diffs) > 0 {
			log.Trace().Any("config", conf).Msg("Schedule change detected")
			changesByGroup[group.GroupName] = change
		}
	}

	return changesByGroup, errs
}

func (s *BroadcastService) sendChangeAlertForGroup(
	ctx context.Context,
	change *model.ScheduleChange,
	chats []*model.Chat,
) (errs []error) {
	text := change.HTML()
	msgParams := bot.SendMessageParams{
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}

	confLight := change.New.Config
	confDark := change.New.Config
	confDark.IsDark = true

	imageFilenameLight, imageDataLight, err := s.scheduleService.PrepareScheduleImage(confLight)
	if err != nil {
		return append(errs, err)
	}
	imageFilenameDark, imageDataDark, err := s.scheduleService.PrepareScheduleImage(confDark)
	if err != nil {
		return append(errs, err)
	}

	for _, chat := range chats {
		conf := confLight
		imageFilename := imageFilenameLight
		imageData := imageDataLight
		if chat.DarkMode {
			conf = confDark
			imageFilename = imageFilenameDark
			imageData = imageDataDark
		}
		if err := s.sendChangeAlert(ctx, conf, msgParams, imageFilename, imageData, chat); err != nil {
			log.Error().Err(err).Any("chat", chat).Msg("Failed to send change alert")
			errs = append(errs, err)
		}
	}

	return errs
}

func (s *BroadcastService) sendChangeAlert(
	ctx context.Context,
	conf model.ScheduleConfig,
	msgParams bot.SendMessageParams,
	imageFilename string,
	imageData []byte,
	chat *model.Chat,
) error {
	msgParams.ChatID = chat.TgChatID
	if _, err := s.SendMessage(ctx, &msgParams); err != nil {
		return err
	}
	if err := botutil.SendSchedulePhoto(ctx, s.Bot, chat, 0, imageFilename, imageData,
		botutil.WeekScheduleMarkup(conf),
	); err != nil {
		log.Error().Err(err).Any("chat", chat).Msg("Failed to send schedule photo")
		return err
	}
	return nil
}

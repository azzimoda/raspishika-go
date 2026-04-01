package service

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service/broadcast"
	"github.com/azzimoda/raspishika-go/internal/service/browser"
	"github.com/azzimoda/raspishika-go/internal/service/schedule"
)

func New(container repository.Container, reporter reporter.Reporter) (*Services, error) {
	s := new(Services)
	s.Reporter = reporter

	var err error
	s.Browser, err = browser.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create browser service: %w", err)
	} else {
		log.Debug().Msg("Created browser service")
	}

	s.Schedule = schedule.New(s.Browser, container.Schedule, container.Group)

	s.Broadcast = broadcast.NewNotificationService(&s.Schedule, s.Browser, container, s.Reporter)

	return s, nil
}

type Services struct {
	Browser     *browser.BrowserService
	Schedule schedule.ScheduleService
	Broadcast   *broadcast.BroadcastService
	Reporter    reporter.Reporter
}

func (s *Services) Close() error { return s.Browser.Close() }

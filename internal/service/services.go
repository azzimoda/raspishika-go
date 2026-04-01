package service

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service/broadcast"
	"github.com/azzimoda/raspishika-go/internal/service/browser"
	"github.com/azzimoda/raspishika-go/internal/service/schedule"
)

func New(db *sqlx.DB, reporter reporter.Reporter) (*Services, error) {
	s := new(Services)
	s.Reporter = reporter

	s.Container = repository.NewContainer(db)

	var err error
	s.Browser, err = browser.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create browser service: %w", err)
	} else {
		log.Debug().Msg("Created browser service")
	}

	s.ScheduleMan = schedule.New(s.Browser, s.Schedule, s.Group)

	s.Broadcast = *broadcast.NewNotificationService(&s.ScheduleMan, *s.Container, s.Reporter)

	return s, nil
}

type Services struct {
	*repository.Container
	Browser     *browser.BrowserService
	ScheduleMan schedule.ScheduleManager
	Broadcast   broadcast.BroadcastService
	Reporter    reporter.Reporter
}

func (s *Services) Close() error { return s.Browser.Close() }

package service

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service/browser"
	smanager "github.com/azzimoda/raspishika-go/internal/service/schedule/manager"
)

func New(db *sqlx.DB, reporter reporter.Reporter) (*Services, error) {
	s := new(Services)
	s.Reporter = reporter

	var err error
	s.Container = repository.NewContainer(db)

	s.Browser, err = browser.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create browser service: %w", err)
	} else {
		log.Debug().Msg("Created browser service")
	}

	return s, nil
}

type Services struct {
	*repository.Container
	Browser     *browser.BrowserService
	ScheduleMan smanager.ScheduleManager
	Reporter    reporter.Reporter
}

func (s *Services) Close() error { return s.Browser.Close() }

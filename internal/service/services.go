package service

import (
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service/browser"
	smanager "github.com/azzimoda/raspishika-go/internal/service/schedule/manager"
)

func New(db *sqlx.DB) (s *Services, err error) {
	s = new(Services)

	s.Repository, err = repository.New(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	} else {
		log.Debug().Msg("Created repository")
	}

	s.Browser, err = browser.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create browser service: %w", err)
	} else {
		log.Debug().Msg("Created browser service")
	}

	return s, nil
}

type Services struct {
	*repository.Repository
	Browser     *browser.BrowserService
	ScheduleMan smanager.ScheduleManager
	Reporter    reporter.Reporter
}

func (s *Services) Close() error { return errors.Join(s.Repository.Close(), s.Browser.Close()) }

package services

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bots/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/services/browser"
	smanager "github.com/azzimoda/raspishika-go/internal/services/schedule/manager"
)

func New() (s *Services, err error) {
	s = new(Services)

	s.Repo, err = repository.New()
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
	Repo     *repository.Repository
	Browser  *browser.BrowserService
	ScheduleMan smanager.ScheduleManager
	Reporter reporter.Reporter
}

func (s *Services) Close() error { return errors.Join(s.Repo.Close(), s.Browser.Close()) }

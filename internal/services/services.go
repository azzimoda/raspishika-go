package services

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/bots/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/services/browser"
	"github.com/azzimoda/raspishika-go/internal/services/cache"
	"github.com/azzimoda/raspishika-go/internal/services/scraper"
)

type Services struct {
	Repo            *repository.Repository
	Browser         *browser.BrowserService
	Cache           *cache.Cache
	ScheduleManager *scraper.ScheduleManager
	Reporter        reporter.Reporter
}

func NewServices() (s *Services, err error) {
	s = &Services{
		Cache:           cache.New(),
		ScheduleManager: scraper.NewScheduleManager(),
	}

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

package services

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/rs/zerolog/log"
)

type Services struct {
	Repo            *database.Repository
	Browser         *browser.BrowserService
	Cache           *cache.Cache
	ScheduleManager *scraper.ScheduleManager
	Reporter        reporter.Reporter
}

func NewServices(cfg *config.MainConfig) (s *Services, err error) {
	s = &Services{
		Cache:           cache.New(&cfg.Cache),
		ScheduleManager: scraper.NewScheduleManager(),
	}

	s.Repo, err = database.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	} else {
		log.Debug().Msg("Created repository")
	}

	s.Browser, err = browser.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create browser service: %w", err)
	} else {
		log.Debug().Msg("Created browser service")
	}

	return s, nil
}

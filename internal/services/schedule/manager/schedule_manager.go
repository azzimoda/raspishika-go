package schedulemanager

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/services/browser"
	"github.com/azzimoda/raspishika-go/internal/services/cache"
	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
)

func NewScheduleManager() *ScheduleManager {
	return &ScheduleManager{sf: singleflight.Group{}}
}

type ScheduleManager struct {
	sf singleflight.Group
}

// Get returns the schedule for the given config and uses cache if available.
func (sm *ScheduleManager) Get(
	repo *repository.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	scheduleCfg scraper.ScheduleConfig,
) (*scraper.RawSchedule, error) {
	// Check cache.
	key := scheduleKey(scheduleCfg)
	if rawScheduleCache, found := cache.C.Get(key); found {
		log.Debug().Str("cacheKey", key).Msg("Cache hit")
		if rawSchedule, ok := rawScheduleCache.(*scraper.RawSchedule); ok {
			return rawSchedule, nil
		} else {
			cache.C.Delete(key)
			log.Error().Type("cacheValueType", rawScheduleCache).Msgf("Invalid cache value type")
			return nil, fmt.Errorf("invalid cache value type: %T", rawScheduleCache)
		}
	}

	// Update cache.
	log.Debug().Str("cacheKey", key).Msg("Cache miss, scraping schedule")
	result, err, _ := sm.sf.Do(key, func() (schedule any, err error) {
		return sm.scrapeSchedule(repo, scheduleCfg, cache, browser)
	})

	if err != nil {
		return nil, err
	}

	// Save cache.
	cache.C.Set(key, result, config.ScheduleTTLDur())

	return result.(*scraper.RawSchedule), nil
}

func (*ScheduleManager) scrapeSchedule(
	repo *repository.Repository,
	config scraper.ScheduleConfig,
	cache *cache.Cache,
	browser *browser.BrowserService,
) (any, error) {
	departmentIDs, err := scraper.FetchDepartmentIDs(repo, browser, cache)
	if err != nil {
		return nil, fmt.Errorf("failed to get department IDs: %w", err)
	}

	url := scraper.ScheduleURL(config, departmentIDs)
	if config.Group != nil {
		return scraper.ScrapeSchedule(url, config)
	} else if config.Teacher != nil {
		return scraper.ScrapeScheduleWithBrowser(browser, url, config)
	} else {
		return nil, fmt.Errorf("invalid schedule config")
	}
}

func scheduleKey(config scraper.ScheduleConfig) string {
	if config.Group != nil {
		return fmt.Sprintf("schedule_%s_%s", config.Group.DepartmentID, config.Group.GroupID)
	} else if config.Teacher != nil {
		return fmt.Sprintf("schedule_%s", config.Teacher.TeacherID)
	} else {
		log.Error().Any("config", config).Msg("Unreachable code")
		return "schedule"
	}
}

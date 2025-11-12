package scraper

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"
)

type ScheduleManagerProvider interface {
	ScheduleManager() *ScheduleManager
}

type ScheduleManager struct {
	sf singleflight.Group
}

func NewScheduleManager() *ScheduleManager {
	return &ScheduleManager{singleflight.Group{}}
}

// Get returns the schedule for the given config and uses cache if available.
func (sm *ScheduleManager) Get(
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	config ScheduleConfig,
) (*RawSchedule, error) {
	// Check cache.
	key := scheduleKey(config)
	if rawScheduleCache, found := cache.C.Get(key); found {
		log.Debug().Str("cacheKey", key).Msg("Cache hit")
		if rawSchedule, ok := rawScheduleCache.(*RawSchedule); ok {
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
		return sm.scrapeSchedule(repo, config, cache, browser)
	})

	if err != nil {
		return nil, err
	}

	// Save cache.
	cache.C.Set(key, result, cache.Config.ScheduleTTLDuration())

	return result.(*RawSchedule), nil
}

func (*ScheduleManager) scrapeSchedule(
	repo *database.Repository,
	config ScheduleConfig,
	cache *cache.Cache,
	browser *browser.BrowserService,
) (any, error) {
	departmentIDs, err := repo.GetDepartmentIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get department IDs: %w", err)
	}

	url := ScheduleURL(config, departmentIDs)
	if config.Group != nil {
		return ScrapeSchedule(cache.Config.Dir, url, config)
	} else if config.Teacher != nil {
		return ScrapeScheduleWithBrowser(browser, url, config)
	} else {
		return nil, fmt.Errorf("invalid schedule config")
	}
}

func scheduleKey(config ScheduleConfig) string {
	if config.Group != nil {
		return fmt.Sprintf("schedule_%s_%s", config.Group.DepartmentID, config.Group.GroupID)
	} else if config.Teacher != nil {
		return fmt.Sprintf("schedule_%s", config.Teacher.TeacherID)
	} else {
		log.Error().Any("config", config).Msg("Unreachable code")
		return "schedule"
	}
}

package schedule

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service/browser"
	"github.com/azzimoda/raspishika-go/internal/service/scraper"
)

func New(scheduleRepo repository.ScheduleRepository, groupRepo repository.GroupRepository) ScheduleManager {
	return ScheduleManager{
		scheduleRepo: scheduleRepo,
		groupRepo:    groupRepo,
		sf:           singleflight.Group{},
	}
}

type ScheduleManager struct {
	scheduleRepo repository.ScheduleRepository
	groupRepo    repository.GroupRepository
	sf           singleflight.Group
}

// Get returns the schedule for the given config and uses cache if available.
func (sm *ScheduleManager) Get(
	browser *browser.BrowserService,
	conf model.ScheduleConfig,
) (*model.RawSchedule, error) {
	key := scheduleKey(conf)
	if rawSchedule, ok := sm.CheckCache(key); ok {
		log.Debug().Str("cacheKey", key).Msg("Cache hit")
		return rawSchedule, nil
	}
	log.Debug().Str("cacheKey", key).Msg("Cache miss")
	return sm.UpdateCache(browser, conf)
}

var ErrNoCache = errors.New("no cache for the key")

func (sm *ScheduleManager) GetCache(conf model.ScheduleConfig) (*model.RawSchedule, error) {
	scheduleCache, err := sm.scheduleRepo.GetByKey(scheduleKey(conf))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoCache
	}
	if err != nil {
		return nil, fmt.Errorf("fail to get cache: %w", err)
	}
	return scheduleCache.Unmarshal()
}

func (sm *ScheduleManager) CheckCache(key string) (rawSchedule *model.RawSchedule, ok bool) {
	scheduleCache, err := sm.scheduleRepo.GetByKey(key)
	if err == nil && scheduleCache.IsActual(config.ScheduleTTLDur()) {
		rawSchedule, err := scheduleCache.Unmarshal()
		return rawSchedule, err == nil
	}
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check schedule cache from DB")
	}
	return nil, false
}

func (sm *ScheduleManager) UpdateCache(
	browser *browser.BrowserService,
	conf model.ScheduleConfig,
) (*model.RawSchedule, error) {
	// Fetch schedule
	key := scheduleKey(conf)
	result, err, _ := sm.sf.Do(key, func() (any, error) {
		return sm.scrapeSchedule(conf, browser)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scrape schedule: %w", err)
	}
	rawSchedule := result.(*model.RawSchedule)

	// Save cache
	scheduleCache, err := model.NewSchedule(key, *rawSchedule)
	if err != nil {
		return rawSchedule, fmt.Errorf("cache not updated: %w", err)
	}

	if err := sm.scheduleRepo.InsertOrUpdate(scheduleCache); err != nil {
		return rawSchedule, fmt.Errorf("cache not updated: %w", err)
	}
	return rawSchedule, nil
}

func (sm *ScheduleManager) scrapeSchedule(
	conf model.ScheduleConfig,
	browser *browser.BrowserService,
) (*model.RawSchedule, error) {
	departmentIDs, err := scraper.FetchDepartmentIDs(sm.groupRepo, browser)
	if err != nil {
		return nil, fmt.Errorf("failed to get department IDs: %w", err)
	}

	url := scraper.ScheduleURL(conf, departmentIDs)
	if conf.Group != nil {
		return scraper.ScrapeSchedule(url, conf)
	} else if conf.Teacher != nil {
		return scraper.ScrapeScheduleWithBrowser(browser, url, conf)
	} else {
		return nil, fmt.Errorf("invalid schedule config")
	}
}

func scheduleKey(config model.ScheduleConfig) string {
	if config.Group != nil {
		return fmt.Sprintf("schedule_%s_%s", config.Group.DepartmentID, config.Group.GroupID)
	} else if config.Teacher != nil {
		return fmt.Sprintf("schedule_%s", config.Teacher.TeacherID)
	} else {
		log.Error().Any("config", config).Msg("Unreachable code")
		return "schedule"
	}
}

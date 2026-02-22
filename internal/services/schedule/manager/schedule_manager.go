package schedulemanager

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/services/browser"
	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
)

type ScheduleManager struct{ sf singleflight.Group }

// Get returns the schedule for the given config and uses cache if available.
func (sm *ScheduleManager) Get(
	repo *repository.Repository,
	browser *browser.BrowserService,
	conf models.ScheduleConfig,
) (*models.RawSchedule, error) {
	key := scheduleKey(conf)
	if rawSchedule, ok := sm.CheckCache(repo, key); ok {
		log.Debug().Str("cacheKey", key).Msg("Cache hit")
		return rawSchedule, nil
	}
	log.Debug().Str("cacheKey", key).Msg("Cache miss")

	return sm.UpdateCache(repo, browser, conf)
}

var ErrNoCache = errors.New("no cache for the key")

func (sm *ScheduleManager) GetCache(
	repo *repository.Repository,
	conf models.ScheduleConfig,
) (*models.RawSchedule, error) {
	scheduleCache, err := models.GetSchedule(repo.DB, scheduleKey(conf))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoCache
	}
	if err != nil {
		return nil, fmt.Errorf("fail to get cache: %w", err)
	}
	return scheduleCache.Unmarshal()
}

func (sm *ScheduleManager) CheckCache(
	repo *repository.Repository,
	key string,
) (rawSchedule *models.RawSchedule, ok bool) {
	scheduleCache, err := models.GetSchedule(repo.DB, key)
	if err == nil && scheduleCache.IsActual(config.ScheduleTTLDur()) {
		rawSchedule, err := scheduleCache.Unmarshal()
		return rawSchedule, err == nil
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to check schedule cache from DB")
	}
	return nil, false
}

func (sm *ScheduleManager) UpdateCache(
	repo *repository.Repository,
	browser *browser.BrowserService,
	scheduleCfg models.ScheduleConfig,
) (*models.RawSchedule, error) {
	// Fetch schedule
	key := scheduleKey(scheduleCfg)
	log.Debug().Str("cacheKey", key).Msg("Cache miss, scraping schedule")
	result, err, _ := sm.sf.Do(key, func() (any, error) {
		return sm.scrapeSchedule(repo, scheduleCfg, browser)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scrape schedule: %w", err)
	}
	rawSchedule := result.(*models.RawSchedule)

	// Save cache
	scheduleCache := models.NewSchedule(key, *rawSchedule)
	if err := scheduleCache.InsertOrUpdate(repo.DB); err != nil {
		return rawSchedule, fmt.Errorf("cache not updated: %w", err)
	}
	return rawSchedule, nil
}

func (*ScheduleManager) scrapeSchedule(
	repo *repository.Repository,
	config models.ScheduleConfig,
	browser *browser.BrowserService,
) (*models.RawSchedule, error) {
	departmentIDs, err := scraper.FetchDepartmentIDs(repo, browser)
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

func scheduleKey(config models.ScheduleConfig) string {
	if config.Group != nil {
		return fmt.Sprintf("schedule_%s_%s", config.Group.DepartmentID, config.Group.GroupID)
	} else if config.Teacher != nil {
		return fmt.Sprintf("schedule_%s", config.Teacher.TeacherID)
	} else {
		log.Error().Any("config", config).Msg("Unreachable code")
		return "schedule"
	}
}

package schedule

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"golang.org/x/sync/singleflight"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service/browser"
	"github.com/azzimoda/raspishika-go/internal/service/scraper"
)

func New(
	browser *browser.BrowserService,
	scheduleRepo repository.ScheduleRepository,
	groupRepo repository.GroupRepository,
) ScheduleManager {
	return ScheduleManager{
		BrowserService: browser,
		scheduleRepo:   scheduleRepo,
		groupRepo:      groupRepo,
		sf:             singleflight.Group{},
	}
}

type ScheduleManager struct {
	*browser.BrowserService
	scheduleRepo repository.ScheduleRepository
	groupRepo    repository.GroupRepository
	sf           singleflight.Group
}

// Get returns the schedule for the given config and uses cache if available.
func (sm *ScheduleManager) Get(conf model.ScheduleConfig) (*model.RawSchedule, error) {
	key := scheduleKey(conf)
	if rawSchedule, ok := sm.CheckCache(key); ok {
		log.Debug().Str("cacheKey", key).Msg("Cache hit")
		return rawSchedule, nil
	}
	log.Debug().Str("cacheKey", key).Msg("Cache miss")
	return sm.UpdateCache(sm.BrowserService, conf)
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

func (sm *ScheduleManager) PrepareScheduleImage(conf model.ScheduleConfig) (fileName string, data []byte, err error) {
	schedule, err := sm.Get(conf)
	if err != nil {
		err = fmt.Errorf("failed loading schedule: %w", err)
		return
	}

	fileName, data, err = sm.htmlToImage(conf, schedule.HTML(config.FetchScheduleTemplate(conf.IsDark)))
	if err != nil {
		return "", nil, err
	}
	return fileName, data, nil
}

func (sm *ScheduleManager) htmlToImage(conf model.ScheduleConfig, html string) (string, []byte, error) {
	imageFileName := path.Join(viper.GetString(config.KeyBrowserScreenshotDir), scheduleScreenshotFileName(conf))
	if err := sm.BrowserService.TakeScreenshotHTML(html, imageFileName); err != nil {
		return "", nil, err
	}

	imageData, err := os.ReadFile(imageFileName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read screenshot: %w", err)
	}
	return imageFileName, imageData, nil
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

func scheduleScreenshotFileName(conf model.ScheduleConfig) string {
	darkSuffix := ""
	if conf.IsDark {
		darkSuffix = "_dark"
	}

	if conf.Group != nil {
		return fmt.Sprintf("schedule_%s%s.png", conf.Group.GroupName, darkSuffix)
	} else if conf.Teacher != nil {
		return fmt.Sprintf("schedule_teacher_%s%s.png", conf.Teacher.Name, darkSuffix)
	} else {
		log.Error().Any("config", conf).Msg("Schedule config is invalid")
		return "schedule.png"
	}
}

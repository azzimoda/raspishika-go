package config

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type ConfigProvider interface {
	Config() *MainConfig
}

type MainConfig struct {
	Telegram struct {
		Token      string `yaml:"token"`
		AdminToken string `yaml:"admin_token"`
		AdminID    int64  `yaml:"admin_id"`
	} `yaml:"telegram"`

	Features struct {
		AdminBot     bool `yaml:"admin_bot"`
		DailySending bool `yaml:"daily_sending"`
		PairSending  bool `yaml:"pair_sending"`
	} `yaml:"features"`

	Database struct {
		File string `yaml:"file"`
	} `yaml:"database"`

	Browser  BrowserConfig `yaml:"browser"`
	Sendings SendingConfig `yaml:"sendings"`
	Cache    CacheConfig   `yaml:"cache"`
	Logger   LoggerConfig  `yaml:"logger"`

	ScheduleTemplateFile string `yaml:"schedule_template_file"`
	ScheduleTemplate     string `yaml:"schedule_template"`
	AdminConfigFile      string `yaml:"admin_config_file"`
}

func (c *MainConfig) EnsureDirs() error {
	dirs := []string{
		path.Dir(c.Database.File),
		c.Browser.ScreenshotDir,
		c.Cache.Dir,
		c.Logger.Dir,
	}
	log.Trace().Strs("dirs", dirs).Msg("Ensuring dirs...")

	for _, dir := range dirs {
		if dir == "" {
			continue
		}

		log.Trace().Str("dir", dir).Msg("Ensuring dir...")
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to make dir '%s': %w", dir, err)
		}
	}
	return nil
}

func (c *MainConfig) LoadTemplate() error {
	data, err := os.ReadFile(c.ScheduleTemplateFile)
	if err != nil {
		return fmt.Errorf("failed to load schedule template: %w", err)
	}
	c.ScheduleTemplate = string(data)
	return nil
}

type BrowserConfig struct {
	Headless      bool   `yaml:"headless"`
	Timeout       int64  `yaml:"timeout"`
	MaxRetries    int    `yaml:"max_retries"`
	ScreenshotDir string `yaml:"screenshot_dir"`
}

type SendingConfig struct {
	// minutes
	PairNotificationTTL int64 `yaml:"pair_notification_ttl"`
}

// PairNotificationTTLDuration retrnus the pair notification TTL as a time.Duration.
//
// Value of SendingConfig.PairNotificationTTL is in minutes, so it is multiplied by time.Minute.
func (sc *SendingConfig) PairNotificationTTLDuration() time.Duration {
	return time.Duration(sc.PairNotificationTTL) * time.Minute
}

type CacheConfig struct {
	Dir         string `yaml:"dir"`
	DefaultTTL  int64  `yaml:"default_ttl"`  // Minutes
	ScheduleTTL int64  `yaml:"schedule_ttl"` // Minutes
	GroupTTL    int64  `yaml:"group_ttl"`    // Days
}

// DefaultTTLDuration retrnus the default TTL as a time.Duration.
//
// Value of CacheConfig.DefaultTTL is in minutes, so it is multiplied by time.Minute.
func (c *CacheConfig) DefaultTTLDuration() time.Duration {
	return time.Duration(c.DefaultTTL) * time.Minute
}

// ScheduleTTLDuration returns the schedule TTL as a time.Duration.
//
// Value of CacheConfig.ScheduleTTL is in minutes, so it is multiplied by time.Minute.
func (c *CacheConfig) ScheduleTTLDuration() time.Duration {
	return time.Duration(c.ScheduleTTL) * time.Minute
}

// GroupTTLDuration returns the group TTL as a time.Duration.
//
// Value of CacheConfig.GroupTTL is in days, so it is multiplied by 24 hours.
func (c *CacheConfig) GroupTTLDuration() time.Duration {
	return time.Duration(c.GroupTTL) * 24 * time.Hour
}

type LoggerConfig struct {
	Level string `yaml:"level"`
	Dir   string `yaml:"dir"`
}

func LoadMainConfig(filename string) (*MainConfig, error) {
	var config MainConfig
	if err := loadConfig(filename, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func LoadCommandsConfig(filename string) (*CommandsConfig, error) {
	var config CommandsConfig
	if err := loadConfig(filename, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadConfig[T any](filename string, config *T) error {
	yamlData, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(yamlData, config); err != nil {
		return fmt.Errorf("failed to unmarshal YAML config: %w", err)
	}
	return nil
}

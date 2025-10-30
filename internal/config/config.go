package config

import (
	"os"
	"path"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram struct {
		Token         string              `yaml:"token"`
		AdminToken    string              `yaml:"admin_token"`
		AdminID       int64               `yaml:"admin_id"`
		MyCommands    []map[string]string `yaml:"my_commands"`
		AdminCommands []map[string]string `yaml:"admin_commands"`
	} `yaml:"telegram"`

	Features struct {
		AdminBot     bool `yaml:"admin_bot"`
		DailySending bool `yaml:"daily_sending"`
		PairSending  bool `yaml:"pair_sending"`
	} `yaml:"features"`

	Database struct {
		File string `yaml:"file"`
	} `yaml:"database"`

	Browser struct {
		Headless      bool   `yaml:"headless"`
		Timeout       int64  `yaml:"timeout"`
		ScreenshotDir string `yaml:"screenshot_dir"`
	} `yaml:"browser"`

	Cache CacheConfig `yaml:"cache"`

	Logger struct {
		Level string `yaml:"level"`
	} `yaml:"logger"`

	ScheduleTemplate string `yaml:"schedule_template"`
}

func (c *Config) EnsureDirs() error {
	dirs := []string{
		path.Dir(c.Database.File),
		c.Browser.ScreenshotDir,
		c.Cache.Dir,
	}
	log.Trace().Strs("dirs", dirs).Msg("Ensuring dirs...")

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

type CacheConfig struct {
	Dir         string `yaml:"dir"`
	DefaultTTL  int64  `yaml:"default_ttl"`  // Minutes
	ScheduleTTL int64  `yaml:"schedule_ttl"` // Minutes
	GroupTTL    int64  `yaml:"group_ttl"`    // Days
}

// DefaultTTLDuration returns the default TTL duration as a time.Duration.
// Value of CacheConfig.DefaultTTL is in minutes, so it is multiplied by time.Minute.
func (c *CacheConfig) DefaultTTLDuration() time.Duration {
	return time.Duration(c.DefaultTTL) * time.Minute
}

// ScheduleTTLDuration returns the schedule TTL duration as a time.Duration.
// Value of CacheConfig.ScheduleTTL is in minutes, so it is multiplied by time.Minute.
func (c *CacheConfig) ScheduleTTLDuration() time.Duration {
	return time.Duration(c.ScheduleTTL) * time.Minute
}

// GroupTTLDuration returns the group TTL duration as a time.Duration.
// Value of CacheConfig.GroupTTL is in days, so it is multiplied by 24 hours.
func (c *CacheConfig) GroupTTLDuration() time.Duration {
	return time.Duration(c.GroupTTL) * 24 * time.Hour
}

func Load(filename string) (*Config, error) {
	yamlData, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(yamlData, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

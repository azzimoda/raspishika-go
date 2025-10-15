package config

import (
	"os"
	"path"

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
		Headless bool  `yaml:"headless"`
		Timeout  int64 `yaml:"timeout"`
	} `yaml:"browser"`
	Cache struct {
		DefaultTTL  int64 `yaml:"default_ttl"`
		ScheduleTTL int64 `yaml:"schedule_ttl"`
		GroupTTL    int64 `yaml:"group_ttl"`
	} `yaml:"cache"`
	Logger struct {
		Level string `yaml:"level"`
	} `yaml:"logger"`
}

func (c *Config) EnsureDirs() error {
	dirs := []string{
		path.Dir(c.Database.File),
	}
	log.Trace().Strs("dirs", dirs).Msg("Ensuring dirs...")

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var Defaults = map[string]any{
	"config_file":   "configs/config.yml",
	"commands_file": "configs/commands.yml",

	"database.file": "databse/db.sqlite3",

	"browser.headless":       false,
	"browser.timeout":        30, // seconds
	"browser.max_retries":    10,
	"browser.screenshot_dir": "storage/cache/screenshots",

	"cache.dir":          "storage/cache",
	"cache.default_ttl":  10, // minutes
	"cache.schedule_ttl": 20, // minutes
	"cache.group_ttl":    7,  // days

	"logger.level": "debug",
	"logger.dir":   "storage/logs",

	"features.admin_bot":     false,
	"features.daily_sending": false,
	"features.pair_sending":  false,

	"pair_sending.notification_ttl": 90, // minutes

	"adminbot.new_chat_report": true,

	"schedule_template_file": "storage/schedule_template.html",
}

func Load() (err error) {
	// Set defaults.
	for key, value := range Defaults {
		viper.SetDefault(key, value)
	}

	if err := ConfigEnv(); err != nil {
		return err
	}
	ConfigFlags()
	if err := LoadFiles(); err != nil {
		return err
	}

	if err := EnsureDirs(); err != nil {
		return err
	}

	// Read schedule template from file.
	data, err := os.ReadFile(viper.GetString("schedule_template_file"))
	if err != nil {
		return fmt.Errorf("failed to read schedule template file: %w", err)
	}
	viper.Set("schedule_template", string(data))

	return nil
}

func ConfigEnv() error {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("failed to load .env file: %w", err)
	}

	viper.SetEnvPrefix("raspishika")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.BindEnv("config_file")
	viper.BindEnv("commands_file")
	viper.BindEnv("telegram_token")
	viper.BindEnv("telegram_admin_token")
	viper.BindEnv("telegram_admin_id")
	viper.AutomaticEnv()

	return nil
}

func ConfigFlags() {
	pflag.String("config", "", "Specify config file")
	pflag.String("commands", "", "Specify commands config file")
	pflag.Time("start", time.Time{}, []string{"2006-01-02T15:04", "01-02T15:04"},
		"Start bot at specified time; format: 2006-01-02T15:04")
	pflag.String("notify", "", "Send specified notification to all chats")
	pflag.String("log", "", "Specify log level")
	pflag.String("log-dir", "", "Specify log directory")
	pflag.Bool("help", false, "Print usage")
	pflag.Parse()

	viper.BindPFlag("config_file", pflag.CommandLine.Lookup("config"))
	viper.BindPFlag("commands_file", pflag.CommandLine.Lookup("commands"))
	viper.BindPFlag("start", pflag.CommandLine.Lookup("start"))
	viper.BindPFlag("notify", pflag.CommandLine.Lookup("notify"))
	viper.BindPFlag("logger.level", pflag.CommandLine.Lookup("log"))
	viper.BindPFlag("logger.dir", pflag.CommandLine.Lookup("log-dir"))
}

func LoadFiles() error {
	filename := viper.GetString("config_file")
	log.Debug().Str("filename", filename).Msg("Loading base config...")
	viper.SetConfigFile(filename)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read base config file: %w", err)
	}

	filename = viper.GetString("commands_file")
	log.Debug().Str("filename", filename).Msg("Loading commands config...")
	viper.SetConfigFile(filename)
	if err := viper.MergeInConfig(); err != nil {
		return fmt.Errorf("failed to read commands config file: %w", err)
	}
	return nil
}

func EnsureDirs() error {
	dirs := []string{
		filepath.Dir(viper.GetString("database.file")),
		viper.GetString("browser.screenshot_dir"),
		viper.GetString("cache.dir"),
		viper.GetString("logger.dir"),
	}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to make dir '%s': %w", dir, err)
		}
	}
	return nil
}

func PrintUsage() bool {
	if f := pflag.Lookup("help"); f != nil && f.Value.String() == "true" {
		pflag.Usage()
		return true
	}
	return false
}

func DefaultTTLDur() time.Duration {
	return viper.GetDuration("cache.default_ttl") * time.Minute
}

func ScheduleTTLDur() time.Duration {
	return viper.GetDuration("cache.schedule_ttl") * time.Minute
}

func GroupTTLDur() time.Duration {
	return viper.GetDuration("cache.group_ttl") * 24 * time.Hour
}

func PairNotificationTTLDur() time.Duration {
	return viper.GetDuration("pair_sending.notification_ttl") * time.Minute
}

func AssertMyCommands(myCommandsAny any) ([]map[string]string, bool) {
	myCommandsArrayAny, ok := myCommandsAny.([]any)
	if !ok {
		return nil, false
	}

	myCommands := make([]map[string]string, len(myCommandsArrayAny))
	for i, cmdAny := range myCommandsArrayAny {
		cmdMapAny, ok := cmdAny.(map[string]any)
		if !ok {
			return nil, false
		}
		cmdMapString := make(map[string]string)
		for k, v := range cmdMapAny {
			cmdMapString[k] = v.(string)
		}

		myCommands[i] = cmdMapString
	}
	return myCommands, true
}

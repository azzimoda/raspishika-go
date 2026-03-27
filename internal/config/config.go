package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

const (
	KeyDotEnv       = "dotenv"
	KeyConfigFile   = "config_file"
	KeyCommandsFile = "commands_file"

	KeyTelegramWorkers    = "telegram.workers"
	KeyTelegramToken      = "Telegram.token"
	KeyTelegramAdminToken = "Telegram.admin_token"
	KeyTelegramAdminId    = "Telegram.admin_id"

	KeyDatabaseFile       = "database.file"
	KeyDatabaseMigrations = "database.migrations"

	KeyBrowserHeadless        = "browser.headless"
	KeyBrowserTimeout         = "browser.timeout"
	KeyBrowserMaxRetries      = "browser.max_retries"
	KeyBrowserScreenshotDir   = "browser.screenshot_dir"
	KeyBrowserWidth           = "browser.width"
	KeyBrowserHeight          = "browser.height"
	KeyBrowserWindowSizeScale = "browser.scale"

	KeyCacheDir         = "cache.dir"
	KeyCacheDefaultTTL  = "cache.default_ttl"
	KeyCacheScheduleTTL = "cache.schedule_ttl"
	KeyCacheGroupTTL    = "cache.group_ttl"

	KeyLoggerLevel = "logger.level"
	KeyLoggerDir   = "logger.dir"

	KeyFeatureAdminBot         = "features.admin_bot"
	KeyFeatureDailySending     = "features.sending.daily"
	KeyFeaturePairNotification = "features.sending.pair_notification"
	KeyFeatureChangeAlert      = "features.sending.change_alert"

	KeySendingWorkers            = "sending.workers"
	KeyPairNotificationTTL       = "sending.pair_notification.ttl"
	KeyChangeAlertUpdateInterval = "sending.change_alert.update_interval"
	KeyAdminNewChatReport        = "adminbot.new_chat_report"

	KeyScheduleTemplateFile     = "schedule_template_file"
	KeyScheduleTemplateDarkFile = "schedule_template_dark_file"
	KeyScheduleTemplate         = "schedule_template"
	KeyScheduleTemplateDark     = "schedule_template_dark"

	KeyStartTime     = "start"
	KeyNotifyMessage = "notify"

	KeyCommandsMain  = "commands.main"
	KeyCommandsAdmin = "commands.admin"
)

var defaults = map[string]any{
	KeyDotEnv:       ".env",
	KeyConfigFile:   "configs/config.yml",
	KeyCommandsFile: "configs/commands.yml",

	KeyTelegramWorkers: 1,

	KeyDatabaseFile:       "database/db.sqlite3",
	KeyDatabaseMigrations: "migrations",

	KeyBrowserHeadless:        true,
	KeyBrowserTimeout:         30, // seconds
	KeyBrowserMaxRetries:      10,
	KeyBrowserScreenshotDir:   "storage/cache/screenshots",
	KeyBrowserWidth:           1280, // px
	KeyBrowserHeight:          720,  // px
	KeyBrowserWindowSizeScale: 1,

	KeyCacheDir:         "storage/cache",
	KeyCacheDefaultTTL:  10, // minutes
	KeyCacheScheduleTTL: 20, // minutes
	KeyCacheGroupTTL:    7,  // days

	KeyLoggerLevel: "debug",
	KeyLoggerDir:   "storage/logs",

	KeyFeatureAdminBot:         false,
	KeyFeatureDailySending:     false,
	KeyFeaturePairNotification: false,
	KeyFeatureChangeAlert:      false,

	KeySendingWorkers:            20,
	KeyPairNotificationTTL:       90, // minutes
	KeyChangeAlertUpdateInterval: 30, // minutes

	KeyAdminNewChatReport: true,

	KeyScheduleTemplateFile:     "storage/templates/schedule_template.html",
	KeyScheduleTemplateDarkFile: "storage/templates/schedule_template_dark.html",
}

func init() {
	for key, value := range defaults {
		viper.SetDefault(key, value)
	}
}

func Load() (err error) {
	ConfigFlags()
	ConfigEnv()
	LoadFiles()

	if err := mkDirs(); err != nil {
		return err
	}

	if _, err = ScheduleTemplate(false); err != nil {
		return fmt.Errorf("failed to load schedule template: %w", err)
	}
	if _, err = ScheduleTemplate(true); err != nil {
		return fmt.Errorf("failed to load schedule template: %w", err)
	}

	return nil
}

func SetDefaults() {
	for key, value := range defaults {
		viper.SetDefault(key, value)
	}
}

func ConfigEnv() {
	dotenvPath := viper.GetString(KeyDotEnv)
	if err := godotenv.Load(dotenvPath); err != nil {
		log.Warn().Err(err).Str("dotenv", dotenvPath).Msg("Failed to load .env file")
	} else {
		log.Debug().Str("dotenv", dotenvPath).Msg(".env file loaded")
	}

	viper.SetEnvPrefix("raspishika")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.BindEnv(KeyConfigFile)
	viper.BindEnv(KeyCommandsFile)
	viper.BindEnv(strings.ReplaceAll(KeyTelegramToken, ".", "_"))
	viper.BindEnv(strings.ReplaceAll(KeyTelegramAdminToken, ".", "_"))
	viper.BindEnv(strings.ReplaceAll(KeyTelegramAdminId, ".", "_"))
	viper.AutomaticEnv()
}

func ConfigFlags() {
	// Parse flags
	pflag.String("env", "", "Specify dotenv file")
	pflag.String("config", "", "Specify config file")
	pflag.String("commands", "", "Specify commands config file")

	pflag.Time("start", time.Time{}, []string{"2006-01-02T15:04", "01-02T15:04"},
		"Start bot at specified time; format: 2006-01-02T15:04")
	pflag.String("notify", "", "Send specified notification to all chats")

	pflag.Bool("headless", true, "Enable browser headless mode")
	pflag.String("log", "", "Specify log level")
	pflag.String("log-dir", "", "Specify log directory")
	pflag.Bool("help", false, "Print usage")
	pflag.Parse()

	// Bind flags
	viper.BindPFlag(KeyDotEnv, pflag.CommandLine.Lookup("env"))
	viper.BindPFlag(KeyConfigFile, pflag.CommandLine.Lookup("config"))
	viper.BindPFlag(KeyCommandsFile, pflag.CommandLine.Lookup("commands"))

	viper.BindPFlag(KeyStartTime, pflag.CommandLine.Lookup("start"))
	viper.BindPFlag(KeyNotifyMessage, pflag.CommandLine.Lookup("notify"))

	viper.BindPFlag(KeyBrowserHeadless, pflag.CommandLine.Lookup("headless"))
	viper.BindPFlag(KeyLoggerLevel, pflag.CommandLine.Lookup("log"))
	viper.BindPFlag(KeyLoggerLevel, pflag.CommandLine.Lookup("log-dir"))
}

func LoadFiles() {
	filename := viper.GetString(KeyConfigFile)
	log.Debug().Str("filename", filename).Msg("Loading base config...")
	viper.SetConfigFile(filename)
	if err := viper.ReadInConfig(); err != nil {
		log.Error().Err(err).Msg("Failed to load main config file! Default values are used.")
	}

	if err := LoadBotCommands(); err != nil {
		log.Error().Err(err).Msg("Failed to load bot commands")
	}
}

func LoadBotCommands() error {
	filename := viper.GetString(KeyCommandsFile)
	log.Debug().Str("filename", filename).Msg("Loading commands config...")

	bytes, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read commands config file: %w", err)
	}

	var commands struct {
		Commands struct {
			MainbotCommands  yaml.Node `yaml:"main"`
			AdminbotCommands yaml.Node `yaml:"admin"`
		} `yaml:"commands"`
	}
	if err := yaml.Unmarshal(bytes, &commands); err != nil {
		return fmt.Errorf("failed to unmarshal commands config file: %w", err)
	}

	mainBotCommands := parseBotCommands(commands.Commands.MainbotCommands)
	adminBotCommands := parseBotCommands(commands.Commands.AdminbotCommands)

	viper.Set(KeyCommandsMain, mainBotCommands)
	viper.Set(KeyCommandsAdmin, adminBotCommands)
	return nil
}

func mkDirs() error {
	dirs := []string{
		filepath.Dir(viper.GetString(KeyDatabaseFile)),
		viper.GetString(KeyBrowserScreenshotDir),
		viper.GetString(KeyCacheDir),
		viper.GetString(KeyLoggerDir),
	}
	log.Trace().Strs("dirs", dirs).Msg("Ensuring directories...")
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

func DefaultTTLDur() time.Duration { return viper.GetDuration(KeyCacheDefaultTTL) * time.Minute }

func ScheduleTTLDur() time.Duration { return viper.GetDuration(KeyCacheScheduleTTL) * time.Minute }

func GroupTTLDur() time.Duration { return viper.GetDuration(KeyCacheGroupTTL) * 24 * time.Hour }

func ScheduleUpdateMonitorInterval() time.Duration {
	return time.Duration(viper.GetInt(KeyChangeAlertUpdateInterval)) * time.Minute
}

func PairNotificationTTLDur() time.Duration {
	return viper.GetDuration(KeyPairNotificationTTL) * time.Minute
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

func FetchScheduleTemplate(is_dark bool) string {
	template, err := ScheduleTemplate(is_dark)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to template")
		key := KeyScheduleTemplate
		if is_dark {
			key = KeyScheduleTemplateDark
		}
		return viper.GetString(key)
	}
	log.Trace().Msg("Loaded schedule template")
	return template
}

func ScheduleTemplate(is_dark bool) (string, error) {
	key := KeyScheduleTemplateFile
	if is_dark {
		key = KeyScheduleTemplateDarkFile
	}

	data, err := os.ReadFile(viper.GetString(key))
	if err != nil {
		return "", fmt.Errorf("failed to read schedule template file: %w", err)
	}
	viper.Set("schedule_template", string(data))
	return string(data), nil
}

func BrowserWindowSizeScaled() (int, int) {
	width := float64(viper.GetInt(KeyBrowserWidth))
	height := float64(viper.GetInt(KeyBrowserHeight))
	scale := viper.GetFloat64(KeyBrowserWindowSizeScale)
	return int(width * scale), int(height * scale)
}

func MainBotCommands() []models.BotCommand {
	return viper.Get(KeyCommandsMain).([]models.BotCommand)
}

func AdminBotCommands() []models.BotCommand {
	return viper.Get(KeyCommandsAdmin).([]models.BotCommand)
}

func parseBotCommands(node yaml.Node) []models.BotCommand {
	var commands []models.BotCommand
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1].Value
		commands = append(commands, models.BotCommand{Command: key, Description: value})
	}
	return commands
}

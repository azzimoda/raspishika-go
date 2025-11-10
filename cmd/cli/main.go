package main

import (
	"flag"
	"os"
	"time"

	"github.com/azzimoda/raspishika-go/internal/app"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/pkg/logger"

	"github.com/rs/zerolog/log"
)

const MainConfigFile = `configs/config.yml`
const CommandsConfigFile = `configs/commands.yml`

type Options struct {
	Help     bool
	Config   string
	LogLevel string
	Start    string
	Notify   string
}

var options Options = Options{
	Help:     false,
	Config:   MainConfigFile,
	LogLevel: "",
	Start:    "",
	Notify:   "",
}

func init() {
	flag.BoolVar(&options.Help, "help", false, "Prints help message")
	flag.StringVar(&options.Config, "config", MainConfigFile, "Path to the main configuration file")
	flag.StringVar(&options.LogLevel, "log-level", "",
		"Log level, overrides config file (trace, debug, info, warn, error, fatal)")
	flag.StringVar(&options.Start, "start", "",
		"Schedules start on the given date/time in format '2006-01-02T15:04' (system time zone)")
	flag.StringVar(&options.Notify, "notify", "",
		"Sends notification to all chats; does not start the bot and features")
}

func main() {
	flag.Parse()

	if options.Help {
		flag.Usage()
		os.Exit(0)
	}

	configFile := MainConfigFile
	if options.Config != "" {
		configFile = options.Config
	}

	mainConfig, err := config.LoadMainConfig(configFile)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to load configuration")
	}
	log.Debug().Str("filename", configFile).Msg("Loaded configuration")

	commandsConfig, err := config.LoadCommandsConfig(CommandsConfigFile)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to load commands configuration")
	}

	if options.LogLevel != "" {
		mainConfig.Logger.Level = options.LogLevel
	}
	logger.SetupLogger(mainConfig.Logger)

	if err := mainConfig.EnsureDirs(); err != nil {
		log.Fatal().Err(err).Msg("Failed to create directories")
	}

	app, err := app.New(mainConfig, commandsConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create application")
	}

	// Check for scheduled start.
	if options.Start != "" {
		startTime, err := time.Parse("2006-01-02T15:04", options.Start)
		startTime = time.Date(
			startTime.Year(), startTime.Month(), startTime.Day(),
			startTime.Hour(), startTime.Minute(), 0, 0,
			time.Now().Location(),
		)

		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse start time")
		}
		log.Info().Msgf("Scheduled start time: %s", startTime.String())
		time.Sleep(time.Until(startTime))
	}

	// Send notification if the option provided.
	if options.Notify != "" {
		app.SendNotification(options.Notify)
		os.Exit(0)
	}

	// Run the application.
	if err := app.Run(); err != nil {
		log.Fatal().Err(err).Msg("Application exited with error")
	}
}

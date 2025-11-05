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

var help = flag.Bool("help", false, "Prints help message.")
var notification = flag.String("notify", "", "Sends notification to all chats; does not start the bot and features.")
var scheduledStartTime = flag.String("start", "", "Schedules start on the given date/time in format '2006-01-02T15:04' (system time zone).")

func main() {
	flag.Parse()

	if help != nil && *help {
		flag.Usage()
		os.Exit(0)
	}

	mainConfig, err := config.LoadMainConfig(MainConfigFile)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to load configuration")
	}
	log.Debug().Msg("Loaded configuration")

	commandsConfig, err := config.LoadCommandsConfig(CommandsConfigFile)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to load commands configuration")
	}

	logger.SetupLogger(mainConfig.Logger)

	if err := mainConfig.EnsureDirs(); err != nil {
		log.Fatal().Err(err).Msg("Failed to create directories")
	}

	app, err := app.New(mainConfig, commandsConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create application")
	}

	if notification != nil && *notification != "" {
		app.SendNotification(*notification)
		os.Exit(0)
	}

	if scheduledStartTime != nil && *scheduledStartTime != "" {
		startTime, err := time.Parse("2006-01-02T15:04", *scheduledStartTime)
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

	if err := app.Run(); err != nil {
		log.Fatal().Err(err).Msg("Application exited with error")
	}
}

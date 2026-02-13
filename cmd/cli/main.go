package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/app"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/pkg/logger"
)

func main() {
	logger.SetupLogger("trace", "")

	if err := config.Load(); err != nil {
		log.Panic().Err(err).Msg("Failed to load configuration")
	}
	if config.PrintUsage() {
		return
	}

	// Reset logget with loaded config.
	logger.SetupLogger(viper.GetString("logger.level"), viper.GetString("logger.dir"))

	log.Debug().Str("baseConfigFile", viper.GetString("config_file")).Msg("Loaded configuration")
	if zerolog.GlobalLevel() == zerolog.TraceLevel {
		log.Trace().Msg("Viper debug:")
		viper.Debug()
		// HACK: The line below prints secrets from env variables. Do not use it in production, only for debugging.
		// log.Trace().Any("settings", viper.AllSettings()).Send()
	}

	app, err := app.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create application")
	}

	// Check for scheduled start.
	if startTime := viper.GetTime("start"); startTime != (time.Time{}) {
		now := time.Now()
		if startTime.Year() == 0 {
			startTime = time.Date(
				now.Year(), startTime.Month(), startTime.Day(),
				startTime.Hour(), startTime.Minute(), 0, 0,
				time.Now().Location(),
			)
		}

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
	if text := viper.GetString("notify"); text != "" {
		app.Notify(text)
		os.Exit(0)
	}

	// Run the application.
	if err := app.Run(); err != nil {
		log.Fatal().Err(err).Msg("Application exited with error")
	}
}

package main

import (
	"flag"
	"time"

	"github.com/azzimoda/raspishika-go/internal/app"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/pkg/logger"

	"github.com/rs/zerolog/log"
)

const ConfigFile = `configs/config.yml`

var scheduledStartTime = flag.String("start", "", "time for scheduled start in format 2006-01-02T15:04")

func main() {
	flag.Parse()

	cfg, err := config.Load(ConfigFile)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to load configuration")
	}
	logger.SetupLogger(cfg.Logger.Level)
	log.Info().Msg("Loaded configuration")

	if err := cfg.EnsureDirs(); err != nil {
		log.Fatal().Err(err).Msg("Failed to create directories")
	}

	app, err := app.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create application")
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

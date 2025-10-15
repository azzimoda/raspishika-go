package main

import (
	"fmt"

	"github.com/azzimoda/raspishika-go/internal/app"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/pkg/logger"

	"github.com/rs/zerolog/log"
)

const ConfigFile = `configs/config.yml`

func main() {
	cfg, err := config.Load(ConfigFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
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

	if err := app.Start(); err != nil {
		log.Fatal().Err(err).Msg("Application exited with error")
	}
}

package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func SetupLogger(level string) {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Timestamp().Caller().Logger()

	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse configured log level; fallback to DEBUG")
		logLevel = zerolog.DebugLevel
	}
	log.Debug().Msgf("Log level: %s", logLevel)
	zerolog.SetGlobalLevel(logLevel)
}

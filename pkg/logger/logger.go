package logger

import (
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/azzimoda/raspishika-go/internal/config"
)

func SetupLogger(cfg config.LoggerConfig) {
	log.Trace().Any("config", cfg).Msg("Setting up logger")

	logLevel, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse configured log level; fallback to DEBUG")
		logLevel = zerolog.DebugLevel
	}

	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}

	if cfg.Dir == "" {
		log.Debug().Msg("No log directory configured; logging to console only")
		log.Logger = zerolog.New(consoleWriter).With().Timestamp().Caller().Logger()
	} else {
		logFileName := filepath.Join(cfg.Dir, "raspishika.log")
		log.Debug().Str("logFile", logFileName).Send()

		log.Logger = zerolog.New(zerolog.MultiLevelWriter(
			consoleWriter,
			&lumberjack.Logger{Filename: logFileName, MaxSize: 16, MaxBackups: 16, MaxAge: 30},
		)).With().Timestamp().Caller().Logger()
	}

	log.Debug().Msgf("Log level: %s", logLevel)
	zerolog.SetGlobalLevel(logLevel)
}

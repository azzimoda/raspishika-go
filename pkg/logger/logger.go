package logger

import (
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

func SetupLogger(logLevelStr, logDir string) {
	log.Trace().Msg("Setting up logger")

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse configured log level; fallback to DEBUG")
		logLevel = zerolog.DebugLevel
	}

	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}

	if logDir == "" {
		log.Trace().Msg("No log directory configured; logging to console only")
		log.Logger = zerolog.New(consoleWriter).With().Timestamp().Caller().Logger()
	} else {
		logFileName := filepath.Join(logDir, "raspishika.log")
		log.Trace().Str("logFile", logFileName).Send()

		log.Logger = zerolog.New(zerolog.MultiLevelWriter(
			consoleWriter,
			&lumberjack.Logger{Filename: logFileName, MaxSize: 16, MaxBackups: 16, MaxAge: 30},
		)).With().Timestamp().Caller().Logger()
	}

	log.Debug().Msgf("Log level: %s", logLevel)
	zerolog.SetGlobalLevel(logLevel)
}

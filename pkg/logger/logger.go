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
	logFileName := filepath.Join(cfg.Dir, "raspishika.log")

	fileWriter := &lumberjack.Logger{
		Filename:   logFileName,
		MaxSize:    16,
		MaxBackups: 16,
		MaxAge:     30,
	}

	logLevel, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse configured log level; fallback to DEBUG")
		logLevel = zerolog.DebugLevel
	}

	multiWriter := zerolog.MultiLevelWriter(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
		fileWriter,
	)
	log.Logger = zerolog.New(multiWriter).With().Timestamp().Caller().Logger()

	log.Debug().Msgf("Log level: %s", logLevel)
	zerolog.SetGlobalLevel(logLevel)
}

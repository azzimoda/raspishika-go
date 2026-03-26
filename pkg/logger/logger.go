package logger

import (
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// SetupLogger configures the logger with the given log level and log directory.
//
// logLevelStr is the log level to use (e.g. "debug", "info", "warn", "error").
// logDir is the directory to write log files to. If empty, logs are written to stderr only.
func SetupLogger(logLevelStr, logDir string) {
	log.Trace().Msg("Setting up logger")

	logLevel, err := zerolog.ParseLevel(logLevelStr)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse configured log level; fallback to DEBUG")
		logLevel = zerolog.DebugLevel
	}

	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}

	if logDir == "" {
		setupConsoleLogger(consoleWriter, logLevel)
	} else {
		logFile := filepath.Join(logDir, "raspishika.log")
		log.Trace().Str("logFile", logFile).Send()

		if _, err := os.Stat(logDir); err == nil {
			// Directory already exists
			setupConsoleFileLogger(consoleWriter, logFile, logLevel)
		} else if err = os.MkdirAll(logDir, 0755); err == nil { // Try to create log directory
			setupConsoleFileLogger(consoleWriter, logFile, logLevel)
		} else {
			log.Error().Msg("Failed to ensure log directory; logging to file disabled")
			setupConsoleLogger(consoleWriter, logLevel)
		}
	}

	log.Debug().Msgf("Log level: %s", logLevel)
	zerolog.SetGlobalLevel(logLevel)
}

func setupConsoleLogger(consoleWriter zerolog.ConsoleWriter, logLevel zerolog.Level) {
	log.Trace().Msg("No log directory configured; logging to console only")
	loggerContext := zerolog.New(consoleWriter).With().Timestamp()
	if logLevel <= zerolog.DebugLevel {
		loggerContext = loggerContext.Caller()
	}
	log.Logger = loggerContext.Logger()
}

func setupConsoleFileLogger(consoleWriter zerolog.ConsoleWriter, logFileName string, logLevel zerolog.Level) {
	loggerContext := zerolog.New(zerolog.MultiLevelWriter(
		consoleWriter,
		&lumberjack.Logger{Filename: logFileName, MaxSize: 16, MaxBackups: 16, MaxAge: 30},
	)).With().Timestamp()
	if logLevel <= zerolog.DebugLevel {
		loggerContext = loggerContext.Caller()
	}
	log.Logger = loggerContext.Logger()
}

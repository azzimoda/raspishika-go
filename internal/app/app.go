package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	adminbot "github.com/azzimoda/raspishika-go/internal/bots/admin"
	"github.com/azzimoda/raspishika-go/internal/bots/admin/reporter"
	mainbot "github.com/azzimoda/raspishika-go/internal/bots/main"
	"github.com/azzimoda/raspishika-go/internal/bots/main/sendings"
	"github.com/azzimoda/raspishika-go/internal/services"
)

func New() (*App, error) {
	s, err := services.NewServices()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}
	app := App{services: s}
	app.services.Reporter = &app

	app.mainBot, err = mainbot.New(app.services)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize main bot: %w", err)
	} else {
		log.Info().Msg("Initialized main bot")
	}

	if viper.GetBool("features.admin_bot") {
		app.adminBot, err = adminbot.New(app.services)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize admin bot")
		} else {
			log.Info().Msg("Initialized admin bot")
		}
	} else {
		log.Info().Msg("Admin bot is disabled")
	}

	return &app, nil
}

type App struct {
	mainBot  *mainbot.MainBot
	adminBot *adminbot.AdminBot
	services *services.Services
}

func (a *App) Run() error {
	startTime := time.Now()
	log.Info().Time("start", startTime).Msg("Starting application...")
	defer func() {
		endTime := time.Now()
		log.Info().Time("end", endTime).TimeDiff("duration", endTime, startTime).Msg("Application stopped")
	}()

	if a.adminBot != nil {
		go a.adminBot.Start()
	}

	a.Report().Log().Temp().Msg("Starting application...")

	go a.mainBot.Start()

	sendingManager := sendings.NewSendingManager(a.mainBot, a.services)

	if viper.GetBool("features.sending.daily") {
		if err := sendingManager.ScheduleDailySending(); err != nil {
			log.Error().Err(err).Msg("Failed to schedule daily sending, skipping")
		} else {
			log.Info().Msg("Daily sending scheduled")
		}
	}

	if viper.GetBool("features.sending.pair") {
		if err := sendingManager.SchedulePairSending(); err != nil {
			log.Error().Err(err).Msg("Failed to schedule pair sending, skipping")
		} else {
			log.Info().Msg("Pair sending scheduled")
		}
	}

	sendingManager.Start()

	a.Report().Log().Msg("Application started.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	a.Shutdown()

	return nil
}

func (a *App) Shutdown() {
	report, err := a.Report().Log().Msg("Shutting down application...")
	if err != nil {
		log.Error().Err(err).Msg("Failed to report shutdown")
	}
	defer report.RemoveMessage()

	if err := a.services.Close(); err != nil {
		a.Report().Log().Err(err).Msg("Services closed with error")
	}

	a.Report().Log().Msg("Application shutdown complete.")
}

func (a *App) Report() reporter.ReportConfig {
	if a.adminBot == nil {
		log.Warn().Msg("Admin bot is not initialized")
		return reporter.ReportConfig{}
	}
	return a.adminBot.Report()
}

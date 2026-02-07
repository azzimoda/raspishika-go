package app

import (
	"context"
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

type App struct {
	MainBot  *mainbot.MainBot
	AdminBot *adminbot.AdminBot
	Services *services.Services
}

func (a *App) Run() error {
	startTime := time.Now()
	log.Info().Time("start", startTime).Msg("Starting application...")
	defer func() {
		endTime := time.Now()
		log.Info().Time("end", endTime).TimeDiff("duration", endTime, startTime).Msg("Application stopped")
	}()

	go a.MainBot.Start()

	if a.AdminBot != nil {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		go a.AdminBot.Start(ctx)
		a.Report().Log().Msg("Starting application...")
	}

	sendingManager := sendings.NewSendingManager(a.MainBot, a.Services)

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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	a.Shutdown()

	return nil
}

func (a *App) Shutdown() {
	log.Info().Msg("Shutting down application...")
	a.Report().Msg("Shutting down application...")

	if err := a.Services.Repo.Close(); err != nil {
		log.Error().Err(err).Msg("Database repository closed with error")
	} else {
		log.Info().Msg("Repository closed")
	}

	a.Services.Browser.Close()
}

func (a *App) Report() reporter.ReportConfig {
	if a.AdminBot == nil {
		log.Warn().Msg("Admin bot is not initialized")
		return reporter.ReportConfig{}
	}
	return a.AdminBot.Report()
}

func New() (*App, error) {
	s, err := services.NewServices()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}
	app := App{Services: s}
	app.Services.Reporter = &app

	app.MainBot, err = mainbot.New(app.Services)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize main bot: %w", err)
	} else {
		log.Info().Msg("Initialized main bot")
	}

	if viper.GetBool("features.admin_bot") {
		app.AdminBot, err = adminbot.New(app.Services)
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

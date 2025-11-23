package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/adminbot"
	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/mainbot"
	"github.com/azzimoda/raspishika-go/internal/mainbot/sendings"
	"github.com/azzimoda/raspishika-go/internal/services"
)

type App struct {
	Config   *config.MainConfig
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
		a.Report().Msg("Starting application...")
	}

	sendingManager := sendings.NewSendingManager(a.Config, a.MainBot, a.Services)

	if a.Config.Features.DailySending {
		if err := sendingManager.ScheduleDailySending(); err != nil {
			log.Error().Err(err).Msg("Failed to schedule daily sending, skipping")
		} else {
			log.Info().Msg("Daily sending scheduled")
		}
	}

	if a.Config.Features.PairSending {
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

func New(cfg *config.MainConfig, commandsCfg *config.CommandsConfig) (*App, error) {
	s, err := services.NewServices(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}
	app := App{Config: cfg, Services: s}
	app.Services.Reporter = &app

	app.MainBot, err = mainbot.New(cfg, commandsCfg.MainBot, app.Services)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize main bot: %w", err)
	} else {
		log.Info().Msg("Initialized main bot")
	}

	if cfg.Features.AdminBot {
		app.AdminBot, err = adminbot.New(cfg, commandsCfg.AdminBot, app.Services)
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

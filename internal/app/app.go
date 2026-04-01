package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	adminbot "github.com/azzimoda/raspishika-go/internal/bot/admin"
	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	mainbot "github.com/azzimoda/raspishika-go/internal/bot/main"
	"github.com/azzimoda/raspishika-go/internal/repository"

	// sending "github.com/azzimoda/raspishika-go/internal/bot/main/sending"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/service"
	"github.com/azzimoda/raspishika-go/pkg/database"
)

func New() (*App, error) {
	app := new(App)
	var err error

	app.db, err = database.New()
	if err != nil {
		return nil, err
	}
	app.container = repository.NewContainer(app.db)

	app.services, err = service.New(app.container, app)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	app.mainBot, err = mainbot.New(app.container, app.services)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize main bot: %w", err)
	} else {
		log.Info().Msg("Initialized main bot")
	}
	app.services.Broadcast.Bot = app.mainBot.Bot

	if viper.GetBool(config.KeyFeatureAdminBot) {
		app.adminBot, err = adminbot.New(app.container, app.services)
		if err != nil {
			log.Error().Err(err).Msg("Failed to initialize admin bot")
		} else {
			log.Info().Msg("Initialized admin bot")
		}
	} else {
		log.Info().Msg("Admin bot is disabled")
	}

	return app, nil
}

type App struct {
	ctx       context.Context
	cancel    context.CancelFunc
	mainBot   *mainbot.MainBot
	adminBot  *adminbot.AdminBot
	db        *sqlx.DB
	container repository.Container
	services  *service.Services
}

func (a *App) Run() error {
	ctx := context.Background()
	a.ctx, a.cancel = context.WithCancel(ctx)

	startTime := time.Now()
	log.Info().Time("start", startTime).Msg("Starting application...")
	defer func() {
		endTime := time.Now()
		log.Info().Time("end", endTime).TimeDiff("duration", endTime, startTime).Msg("Application stopped")
	}()

	if a.adminBot != nil {
		go a.adminBot.Start()
	}

	report, err := a.Report().Log().Temp().Msg("Starting application...")

	go a.mainBot.Start()

	a.startServices(a.ctx)

	if err == nil {
		report.RemoveMessage()
	}
	a.Report().Log().Msgf("Application started on bot @%s.", a.mainBot.Me.Username)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	a.Shutdown()

	return nil
}

func (a *App) startServices(ctx context.Context) {
	if viper.GetBool(config.KeyFeatureDailySending) {
		if err := a.services.Broadcast.ScheduleDaily(); err != nil {
			log.Error().Err(err).Msg("Failed to schedule daily sending, skipping")
		} else {
			log.Info().Msg("Daily sending scheduled")
		}
	} else {
		log.Info().Msg("Daily sending disabled")
	}

	if viper.GetBool(config.KeyFeaturePairNotification) {
		if err := a.services.Broadcast.SchedulePairNotification(); err != nil {
			log.Error().Err(err).Msg("Failed to schedule pair sending, skipping")
		} else {
			log.Info().Msg("Pair sending scheduled")
		}
	} else {
		log.Info().Msg("Pair notification disabled")
	}

	if viper.GetBool(config.KeyFeatureChangeAlert) {
		go a.services.Broadcast.RunChangeAlertNotifier(ctx)
		log.Info().Msg("Change alert notifier started")
	} else {
		log.Info().Msg("Change alert disabled")
	}

	a.services.Broadcast.Start()
}

func (a *App) Shutdown() {
	a.cancel()

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
		return reporter.NewReportConfig(nil, viper.GetInt64(config.KeyTelegramAdminId))
	}
	return a.adminBot.Report()
}

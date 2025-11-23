package mainbot

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/services"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
)

func New(
	cfg *config.MainConfig,
	myCommands []map[string]string,
	services *services.Services,
) (mb *MainBot, err error) {
	mb = &MainBot{
		config:     cfg,
		myCommands: myCommands,
		services:   services,
	}

	opts := []bot.Option{
		bot.WithMiddlewares(
			mb.ignoreOldMessagesMiddleware,
			mb.callbackQuerySingleFlightMiddleware,
			mb.ensureChatMiddleware,
			mb.logMiddleware,
		),
		bot.WithDefaultHandler(mb.defaultHandler),
		// bot.WithErrorsHandler(mb.errorsHandler),
	}
	mb.Bot, err = bot.New(cfg.Telegram.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	me, err := mb.Bot.GetMe(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get bot info: %w", err)
	}
	mb.Me = me

	mb.registerHandlers()

	return mb, nil
}

type MainBot struct {
	config     *config.MainConfig
	myCommands []map[string]string
	Bot        *bot.Bot
	Me         *models.User
	services   *services.Services
}

func (mb *MainBot) Start() {
	success, err := tgbothelpers.SetMyCommands(context.Background(), mb.Bot, mb.myCommands)
	if err != nil {
		log.Error().Err(err).Msg("Error while trying to set my commands")
	}
	if !success {
		log.Error().Msg("Failed to set my commands")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	mb.Bot.Start(ctx)
	log.Info().Msg("Main bot started")
}

func (mb *MainBot) Report() reporter.ReportConfig {
	if mb.services.Reporter == nil {
		return reporter.ReportConfig{}
	}
	return mb.services.Reporter.Report()
}

// func (mb *MainBot) errorsHandler(err error) {
// } ->

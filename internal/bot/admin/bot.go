package adminbot

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/service"
)

func New(services *service.Services) (*AdminBot, error) {
	ab := AdminBot{services: services}

	opts := []bot.Option{
		bot.WithMiddlewares(ab.filterNotAdminMiddleware),
		bot.WithDefaultHandler(ab.defaultHandler),
	}

	var err error
	ab.Bot, err = bot.New(viper.GetString(config.KeyTelegramAdminToken), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create new bot: %w", err)
	}

	ab.registerHandlers()

	return &ab, nil
}

type AdminBot struct {
	*bot.Bot
	services *service.Services
}

func (b *AdminBot) Start() {
	log.Info().Msg("Starting admin bot...")
	ctx := context.Background()

	success, err := b.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: config.AdminBotCommands()})
	if err != nil || !success {
		log.Error().Err(err).Msg("Failed to set my commands")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	b.Bot.Start(ctx)
}

func (b *AdminBot) Report() reporter.ReportConfig {
	log.Trace().Msg("Reporting...")
	return reporter.NewReportConfig(b.Bot, viper.GetInt64(config.KeyTelegramAdminId))
}

func (ab *AdminBot) filterNotAdminMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		if update.Message != nil {
			if update.Message.Chat.Type != "private" || update.Message.Chat.ID != viper.GetInt64(config.KeyTelegramAdminId) {
				log.Trace().Msgf("Ignoring update from chat %d", update.Message.Chat.ID)
				return
			}

			log.Trace().Msgf("Processing update from admin chat %d", update.Message.Chat.ID)
		} else {
			log.Trace().Msg("Ignoring update")
			return
		}

		next(ctx, b, update)
	}
}

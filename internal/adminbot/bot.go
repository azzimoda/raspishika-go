package adminbot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/services"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

type AdminBot struct {
	bot      *bot.Bot
	services *services.Services
}

func (b *AdminBot) API() *bot.Bot {
	return b.bot
}

func (b *AdminBot) Start(ctx context.Context) {
	log.Info().Msg("Starting admin bot...")
	myCommands, ok := config.AssertMyCommands(viper.Get("adminbot_commands"))
	if !ok {
		log.Error().Msg("Failed to assert adminbot_commands")
	}

	success, err := tgbothelpers.SetMyCommands(context.Background(), b.bot, myCommands)
	if err != nil {
		log.Error().Err(err).Msg("Failed to set my commands")
	}
	if !success {
		log.Error().Msg("Failed to set my commands")
	}
	b.bot.Start(ctx)
}

func (b *AdminBot) Report() reporter.ReportConfig {
	log.Trace().Msg("Reporting...")
	return reporter.NewReportConfig(b.bot, viper.GetInt64("telegram.admin_id"))
}

func (b *AdminBot) ReportNewChat(chat *database.Chat) {
	if !viper.GetBool("adminbot.new_chat_report") {
		log.Trace().Msg("New chat report is disabled.")
		return
	}
	b.Report().Chat(chat.TgChatID).
		Msgf("Registered new chat with group %s.", utils.DerefOrTypeDefault(chat.GroupName))
}

func New(services *services.Services) (*AdminBot, error) {
	ab := AdminBot{services: services}

	opts := []bot.Option{
		bot.WithMiddlewares(ab.filterNotAdminMiddleware),
		bot.WithDefaultHandler(ab.defaultHandler),
	}

	var err error
	ab.bot, err = bot.New(viper.GetString("telegram.admin_token"), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create new bot: %w", err)
	}

	ab.registerHandlers()

	return &ab, nil
}

func (ab *AdminBot) filterNotAdminMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message != nil {
			if update.Message.Chat.Type != "private" || update.Message.Chat.ID != viper.GetInt64("telegram.admin_id") {
				log.Trace().Msgf("Ignoring update from chat %d", update.Message.Chat.ID)
				return
			}
		}
		log.Trace().Msgf("Processing update from admin chat %d", update.Message.Chat.ID)
		next(ctx, b, update)
	}
}

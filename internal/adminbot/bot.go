package adminbot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/services"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

type AdminBot struct {
	config      *config.MainConfig
	myCommands  []map[string]string
	adminConfig *config.AdminConfig
	bot         *bot.Bot
	services    *services.Services
}

func (b *AdminBot) API() *bot.Bot {
	return b.bot
}

func (b *AdminBot) Start(ctx context.Context) {
	log.Info().Msg("Starting admin bot...")
	success, err := tgbothelpers.SetMyCommands(context.Background(), b.bot, b.myCommands)
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
	return reporter.NewReportConfig(b.bot, b.config.Telegram.AdminID)
}

func (b *AdminBot) ReportNewChat(chat *database.Chat) {
	if !b.adminConfig.NewChatReport {
		log.Trace().Msg("New chat report is disabled.")
		return
	}
	b.Report().Chat(chat.TgChatID).
		Msgf("Registered new chat with group %s.", utils.DerefOrTypeDefault(chat.GroupName))
}

func New(
	cfg *config.MainConfig,
	myCommands []map[string]string,
	services *services.Services,
) (*AdminBot, error) {
	adminCfg, err := config.LoadAdminConfig(cfg.AdminConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin config: %w", err)
	}
	ab := AdminBot{
		config:      cfg,
		myCommands:  myCommands,
		adminConfig: adminCfg,
		services:    services,
	}

	opts := []bot.Option{
		bot.WithMiddlewares(ab.filterNotAdminMiddleware),
		bot.WithDefaultHandler(ab.defaultHandler),
	}
	ab.bot, err = bot.New(cfg.Telegram.AdminToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create new bot: %w", err)
	}

	ab.registerHandlers()

	return &ab, nil
}

func (ab *AdminBot) filterNotAdminMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message != nil {
			if update.Message.Chat.Type != "private" || update.Message.Chat.ID != ab.config.Telegram.AdminID {
				log.Trace().Msgf("Ignoring update from chat %d", update.Message.Chat.ID)
				return
			}
		}
		log.Trace().Msgf("Processing update from admin chat %d", update.Message.Chat.ID)
		next(ctx, b, update)
	}
}

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
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

type AdminBot struct {
	ctx         context.Context
	config      *config.MainConfig
	myCommands  []map[string]string
	adminConfig *config.AdminConfig
	bot         *bot.Bot
	services    *services.Services
}

func (b *AdminBot) API() *bot.Bot {
	return b.bot
}

func (b *AdminBot) Start() {
	log.Info().Msg("Starting admin bot...")
	// tgbot.SetMyCommands(b.bot, b.myCommands)
	b.bot.SetMyCommands(b.ctx, &bot.SetMyCommandsParams{
		Commands: []models.BotCommand{},
	})
	b.bot.Start(b.ctx)
}

func (b *AdminBot) Stop() {
	// TODO
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
		Sendf("Registered new chat with group %s.", utils.DerefOrTypeDefault(chat.GroupName))
}

func New(
	ctx context.Context,
	cfg *config.MainConfig,
	myCommands []map[string]string,
	services *services.Services,
) (*AdminBot, error) {

	adminCfg, err := config.LoadAdminConfig(cfg.AdminConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin config: %w", err)
	}

	ab := AdminBot{
		ctx:         ctx,
		config:      cfg,
		myCommands:  myCommands,
		adminConfig: adminCfg,
		services:    services,
	}

	opts := []bot.Option{
		bot.WithMiddlewares(ab.filterNotAdminMiddleware),
		bot.WithDefaultHandler(ab.defaultHandler),
	}
	b, err := bot.New(cfg.Telegram.AdminToken, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create new bot: %w", err)
	}
	ab.bot = b

	ab.registerHandlers()

	return &ab, nil
}

func (ab *AdminBot) filterNotAdminMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message != nil {
			if update.Message.Chat.Type != "private" || update.Message.Chat.ID != ab.config.Telegram.AdminID {
				return
			}
		}
		next(ctx, b, update)
	}
}

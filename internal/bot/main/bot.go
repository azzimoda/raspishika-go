package mainbot

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/bot/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/bot/botutil"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/internal/service"
)

func New(container repository.Container, services *service.Services) (mb *MainBot, err error) {
	mb = &MainBot{container: container, services: services}

	proxy, err := botutil.FindAvailableProxy()
	if err != nil {
		return nil, fmt.Errorf("failed to find available proxy: %w", err)
	}

	httpClient, err := botutil.NewHTTPProxyClient(proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client: %w", err)
	}

	opts := []bot.Option{
		bot.WithHTTPClient(30*time.Second, httpClient),
		bot.WithMiddlewares(
			mb.ignoreOldMessagesMiddleware,
			mb.ignoreInaccessibleMessageCQMiddleware,
			mb.callbackQuerySingleFlightMiddleware,
			mb.ensureChatMiddleware,
			mb.logMiddleware,
		),
		bot.WithWorkers(viper.GetInt(config.KeyTelegramWorkers)),
		bot.WithDefaultHandler(mb.defaultHandler),
		bot.WithCheckInitTimeout(30 * time.Second),
	}
	if viper.GetString(config.KeyLoggerLevel) == "trace" {
		opts = append(opts, bot.WithDebug())
	}
	mb.Bot, err = bot.New(viper.GetString(config.KeyTelegramToken), opts...)
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
	*bot.Bot
	Me        *tgmodels.User
	container repository.Container
	services  *service.Services
}

func (mb *MainBot) Start() {
	log.Trace().Any("mainbot_commands", viper.Get("mainbot_commands")).Send()
	ctx := context.Background()

	success, err := mb.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands:     config.MainBotCommands(),
		LanguageCode: "ru",
	})
	if err != nil || !success {
		log.Error().Err(err).Msg("Error while trying to set my commands")
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

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

	"github.com/azzimoda/raspishika-go/internal/bots/admin/reporter"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/services"
	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

func New(
	services *services.Services,
) (mb *MainBot, err error) {
	mb = &MainBot{
		services: services,
	}

	opts := []bot.Option{
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
	Bot      *bot.Bot
	Me       *tgmodels.User
	services *services.Services
}

func (mb *MainBot) Start() {
	log.Trace().Any("mainbot_commands", viper.Get("mainbot_commands")).Send()
	myCommands, ok := config.AssertMyCommands(viper.Get("mainbot_commands"))
	if !ok {
		log.Error().Msg("Failed to assert mainbot_commands")
	}

	success, err := bothelpers.SetMyCommands(context.Background(), mb.Bot, myCommands)
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

// FetchGroupByNameWithValidation tries to validate given group name and fetch group from the database.
//
// When given group name is not found in database, it fetches group from the website and
// updated the database, then tries again. If group is not found after successful update, it returns ErrGroupNotFound.
// When any other error occurs, it returns the error.
func (mb *MainBot) FetchGroupByNameWithValidation(name models.GroupName) (*models.Group, error) {
	groupName, err := models.ValidateGroupName(mb.services.Repository.DB, name)
	if err != nil {
		return nil, err
	}

	if groupName, err = models.ValidateGroupNameCase(mb.services.Repository.DB, groupName); err != nil {
		log.Warn().Err(err).Msg("Updating groups")
		// Try to update groups.
		if _, err := scraper.FetchGroups(mb.services.Repository, mb.services.Browser); err != nil {
			return nil, fmt.Errorf("failed to fetch groups: %w", err)
		}

		// Try again.
		if groupName, err = models.ValidateGroupNameCase(mb.services.Repository.DB, groupName); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrGroupNotFound, err)
		}
	} else {
		log.Trace().Any("given", name).Any("groupName", groupName).Bool("give == validated", name == groupName).
			Msg("Group name case is validated")
	}

	// Group found.
	group, err := models.GetGroupByName(mb.services.Repository.DB, groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get group by validated name (%s): %w", groupName, err)
	}
	return group, nil
}

package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/tgbot"
	"github.com/azzimoda/raspishika-go/pkg/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

const (
	ErrMsgTryLater            = "Произошла ошибка, попробуйте позже"
	ErrMsgFailedFetchSchedule = "Не удалось загрузить расписание, попробуйте позже"
	ErrMsgSelectGroupAgain    = "Не удалось найти группу, выберите группу ещё раз"
)

var (
	ErrWrongGroupNameFormat = errors.New("wrong group name format")
	ErrGroupNotFound        = errors.New("group not found")
)

type BotManager interface {
	config.ConfigProvider
	tgbot.BotAPIProvider
	database.RepositoryProvider
	browser.BrowserServiceProvider
	cache.CacheProvider
	reporter.Reporter
}

func SendErrorMessage(api *tgbotapi.BotAPI, tgChatID int64, text string) error {
	err := tgbot.SendTempMessage(
		api,
		tgChatID,
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, text),
		10*time.Second,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to send error message")
	}
	return err
}

func InlineButtonMarkupUpdate(kind string, groupName string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Обновить", "update_"+kind+"\n"+groupName)))
}

func ScheduleScreenshotFileName(config scraper.ScheduleConfig) string {
	if config.Group != nil {
		return fmt.Sprintf("schedule_%s.png", config.Group.GroupName)
	} else if config.Teacher != nil {
		return fmt.Sprintf("schedule_%s.png", config.Teacher.Name)
	} else {
		panic("unreachable")
	}
}

func AccessMenuInlineMarkup(accessLevel int) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("0", "set_access\n0"),
			tgbotapi.NewInlineKeyboardButtonData("1", "set_access\n1"),
			tgbotapi.NewInlineKeyboardButtonData("2", "set_access\n2"),
		},
		{tgbotapi.NewInlineKeyboardButtonData("Закрыть", "delete")},
	}
	for i := range 3 {
		if i == accessLevel {
			rows[0][i].Text = fmt.Sprintf("[%d]", i)
		}
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return markup
}

// MainMenuReplyMarkup returns a ReplyKeyboardMarkup or a ReplyKeyboardRemove object
// depending on the isPrivate parameter.
// If isPrivate is true, a ReplyKeyboardMarkup is returned, else a ReplyKeyboardRemove is returned.
func MainMenuReplyMarkup(isPrivate bool) any {
	if isPrivate {
		markup := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Сегодня"),
				tgbotapi.NewKeyboardButton("Завтра"),
				tgbotapi.NewKeyboardButton("Неделя"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Другая группа"),
				tgbotapi.NewKeyboardButton("Преподаватель"),
			),
		)
		markup.ResizeKeyboard = true
		return markup
	} else {
		return tgbotapi.NewRemoveKeyboard(false)
	}
}

// FetchGroupByNameWithVadiation tries to validate given group name and fetch group from the database.
//
// When the group name format cannot be validated, it returns ErrWrongGroupNameFormat.
// When given group name is not found in database, it fetches group from the website and
// updated the database, then tries again. If group is not found after successful update, it returns ErrGroupNotFound.
// When any other error occurs, it returns the error.
func FetchGroupByNameWithVadiation(
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	name string,
) (*database.Group, error) {
	groupName, err := utils.ValidateGroupNameFormat(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWrongGroupNameFormat, err)
	}

	if groupName, err = repo.ValidateGroupNameCase(groupName); err != nil {
		// Try to update groups.
		if _, err := scraper.FetchGroups(repo, browser, cache); err != nil {
			return nil, fmt.Errorf("failed to fetch groups: %w", err)
		}

		// Try again.
		if groupName, err = repo.ValidateGroupNameCase(groupName); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrGroupNotFound, err)
		}
	}

	// Group found.
	group, err := repo.GetGroupByName(groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get group by name (%s) after successful update: %w", groupName, err)
	}
	return group, nil
}

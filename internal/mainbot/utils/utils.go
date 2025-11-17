package utils

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
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
	tgbothelpers.BotAPIProvider
	database.RepositoryProvider
	browser.BrowserServiceProvider
	cache.CacheProvider
	reporter.Reporter
	scraper.ScheduleManagerProvider
}

func SendErrorMessage(api *tgbotapi.BotAPI, tgChatID int64, text string) error {
	err := tgbothelpers.SendTempMessageOld(
		api,
		tgChatID,
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, text),
		10*time.Second,
	)
	if err != nil {
		log.Error().Err(err).Int64("tgChatID", tgChatID).Msg("Failed to send error message")
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

// SettingsMenuMessage returns a MessageConfig object with settings menu message.
//
// The message includes the current group name, daily sending time, and pair notification settings.
// Its reply markup is a InlineKeyboardMarkup object with settings options.
func SettingsMenuMessage(chat *database.Chat) tgbotapi.MessageConfig {
	dailyTime := "выключено"
	if chat.DailySendingTime != nil {
		dailyTime = *chat.DailySendingTime
	}
	pairNotification := "выключено"
	if chat.PairSending {
		pairNotification = "включено"
	}

	text := fmt.Sprintf(`Меню настроек

Группа: %s
Ежедневная рассылка: %s
Напоминания перед парами: %s`,
		utils.DerefOrTypeDefault(chat.GroupName),
		dailyTime,
		pairNotification,
	)
	if !chat.IsPrivate() {
		text += fmt.Sprintf("\nУровень доступа: %d", chat.Access)
	}

	newMsg := tgbotapi.NewMessage(chat.TgChatID, text)
	newMsg.ReplyMarkup = SettingsInlineMarkup(chat)
	return newMsg
}

// SettingsInlineMarkup returns an InlineKeyboardMarkup object with settings options.
func SettingsInlineMarkup(chat *database.Chat) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0)

	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Изменить группу", "config_group"),
	})

	if chat.DailySendingTime == nil {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Включить ежедневную рассылку", "config_daily_time"),
		})
	} else {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Изменить время рассылки", "config_daily_time"),
			tgbotapi.NewInlineKeyboardButtonData("Выключить ежедневную рассылку", "daily_off"),
		})
	}

	if chat.PairSending {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Выключить напоминания", "config_reminder\nfalse"),
		})
	} else {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Включить напоминания", "config_reminder\ntrue"),
		})
	}

	if !chat.IsPrivate() {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("0", "config_access\n0"),
			tgbotapi.NewInlineKeyboardButtonData("1", "config_access\n1"),
			tgbotapi.NewInlineKeyboardButtonData("2", "config_access\n2"),
		})
		for i := range 3 {
			if i == chat.Access {
				rows[len(rows)-1][i].Text = fmt.Sprintf("[%d]", i)
			}
		}
	}

	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Закрыть", "delete"),
	})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// FetchGroupByNameWithValidation tries to validate given group name and fetch group from the database.
//
// When the group name format cannot be validated, it returns ErrWrongGroupNameFormat.
// When given group name is not found in database, it fetches group from the website and
// updated the database, then tries again. If group is not found after successful update, it returns ErrGroupNotFound.
// When any other error occurs, it returns the error.
func FetchGroupByNameWithValidation(
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
		log.Warn().Err(err).Msg("Updating groups")
		// Try to update groups.
		if _, err := scraper.FetchGroups(repo, browser, cache); err != nil {
			return nil, fmt.Errorf("failed to fetch groups: %w", err)
		}

		// Try again.
		if groupName, err = repo.ValidateGroupNameCase(groupName); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrGroupNotFound, err)
		}
	} else {
		log.Trace().Str("given", name).Str("groupName", groupName).
			Bool("give == validated", name == groupName).
			Msg("Group name case is validated")
	}

	// Group found.
	group, err := repo.GetGroupByName(groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get group by validated name (%s): %w", groupName, err)
	}
	return group, nil
}

func HandleTelegramAPIError(repo *database.Repository, tgErr *tgbotapi.Error, chat *database.Chat) error {
	if strings.Contains(strings.ToLower(tgErr.Message), "forbidden") {
		log.Warn().Int64("tgChatID", chat.TgChatID).Msg("Forbidden, deleting chat...")
		repo.DeleteChat(chat.ID)
		return nil
	}

	if tgErr.MigrateToChatID != 0 {
		log.Warn().Int64("tgChatID", chat.TgChatID).Int64("migrateToChatID", tgErr.MigrateToChatID).
			Msg("Chat migrated, updating chat ID...")

		if c, err := repo.GetChatByTgChatID(tgErr.MigrateToChatID); err == nil {
			log.Warn().Int64("tgChatID", chat.TgChatID).Int64("migrateToChatID", tgErr.MigrateToChatID).
				Msg("Chat already exists, deleting old chat...")
			if err := repo.DeleteChat(c.ID); err != nil {
				return fmt.Errorf("failed to delete old chat: %w", err)
			}
		} else {
			if err := repo.UpdateChatTgChatID(chat.ID, tgErr.MigrateToChatID); err != nil {
				log.Error().Err(err).Int64("tgChatID", chat.TgChatID).Int64("migrateToChatID", tgErr.MigrateToChatID).
					Msg("Failed to update chat ID")
				return fmt.Errorf("failed to update chat ID: %w", err)
			}
		}

		return nil
	}

	return fmt.Errorf("telegram API error: %w", tgErr)
}

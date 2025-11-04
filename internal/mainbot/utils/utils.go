package utils

import (
	"fmt"
	"time"

	"github.com/azzimoda/raspishika-go/internal/adminbot/reporter"
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/scraper"
	"github.com/azzimoda/raspishika-go/pkg/tgbot"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

const (
	ErrMsgTryLater            = "Произошла ошибка, попробуйте позже"
	ErrMsgFailedFetchSchedule = "Не удалось загрузить расписание, попробуйте позже"
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
	err := tgbot.SendTempMessage(api, tgChatID, text, 10*time.Second)
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

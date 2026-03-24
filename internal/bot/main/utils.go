package mainbot

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/pkg/bothelper"
)

func (mb *MainBot) PrepareScheduleImage(conf model.ScheduleConfig) (fileName string, data []byte, err error) {
	schedule, err := mb.services.ScheduleMan.Get(mb.services.Repository, mb.services.Browser, conf)
	if err != nil {
		err = fmt.Errorf("failed loading schedule: %w", err)
		return
	}

	fileName, data, err = mb.htmlToImage(conf, schedule.HTML(config.FetchScheduleTemplate(conf.IsDark)))
	if err != nil {
		return "", nil, err
	}
	return fileName, data, nil
}

func (mb *MainBot) htmlToImage(conf model.ScheduleConfig, html string) (string, []byte, error) {
	imageFileName := path.Join(viper.GetString(config.KeyBrowserScreenshotDir), scheduleScreenshotFileName(conf))
	if err := mb.services.Browser.TakeScreenshotHTML(html, imageFileName); err != nil {
		return "", nil, err
	}

	imageData, err := os.ReadFile(imageFileName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read screenshot: %w", err)
	}
	return imageFileName, imageData, nil
}

// addContextHandlerError adds an error to the handler error context.
func addContextHandlerError(ctx context.Context, err error) {
	handlerErrs, ok := ctx.Value(errorContextKey).(*[]error)
	if ok {
		if err != nil {
			*handlerErrs = append(*handlerErrs, err)
		}
	} else {
		log.Warn().Err(err).Msg("Error context not found")
	}
}

// mainMenuReplyMarkup returns the main menu keyboard for the given chat type.
func mainMenuReplyMarkup(isPrivate bool) tgmodels.ReplyMarkup {
	if isPrivate {
		return tgmodels.ReplyKeyboardMarkup{
			Keyboard: [][]tgmodels.KeyboardButton{
				{{Text: "Неделя"}},
				{{Text: "Сегодня"}, {Text: "Завтра"}, {Text: "Преподаватель"}},
			},
			ResizeKeyboard: true,
		}
	} else {
		return tgmodels.ReplyKeyboardRemove{RemoveKeyboard: true}
	}
}

func sendErrorMessage(ctx context.Context, b *bot.Bot, params *bot.SendMessageParams) error {
	err := bothelpers.SendTempMessage(ctx, b, 7*time.Second, params)
	if err != nil {
		log.Error().Err(err).Any("params", params).Msg("Failed to send error message")
	}
	return err
}

func shortenText(text string, maxLength int) string {
	if len(text) > maxLength {
		return text[:maxLength-2] + "…"
	}
	return text
}

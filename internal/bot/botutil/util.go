package botutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"

	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/pkg/bothelper"
)

const (
	ErrMsgTryLater             = "Произошла ошибка, попробуйте позже"
	ErrMsgCouldNotLoadSchedule = "Не удалось загрузить расписание, попробуйте позже"
	ErrMsgCouldNotUpdateData   = "Не удалось обновить данные, попробуйте позже"
	ErrMsgCouldNotSendSchedule = "Не удалось отправить расписание, попробуте позже"
	ErrMsgSelectGroupAgain     = "Не удалось найти группу, выберите группу ещё раз"
)

func SendWeekScheduleMessages(
	ctx context.Context,
	b *bot.Bot,
	messageThreadID int,
	chat *model.Chat,
	conf model.ScheduleConfig,
	imageFilename string,
	imageData []byte,
) error {
	var errs []error

	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          chat.TgChatID,
		MessageThreadID: messageThreadID,
		Text:            conf.FormatHTML() + ":",
		ParseMode:       tgmodels.ParseModeHTML,
		ReplyMarkup:     MainMenuReplyMarkup(chat.IsPrivate()),
	}); err != nil {
		errs = append(errs, err)
	}

	if _, err := b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID:          chat.TgChatID,
		MessageThreadID: messageThreadID,
		Action:          tgmodels.ChatActionUploadPhoto,
	}); err != nil {
		errs = append(errs, err)
	}

	replyMarkup := WeekScheduleMarkup(conf)
	if err := SendSchedulePhoto(ctx, b, chat, messageThreadID, imageFilename, imageData, replyMarkup); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// MainMenuReplyMarkup returns the main menu keyboard for the given chat type.
func MainMenuReplyMarkup(isPrivate bool) tgmodels.ReplyMarkup {
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

func SendSchedulePhoto(
	ctx context.Context,
	b *bot.Bot,
	chat *model.Chat,
	messageThreadID int,
	imageFilename string,
	imageData []byte,
	replyMarkup tgmodels.ReplyMarkup,
) error {
	log.Trace().Any("tgChatID", chat.TgChatID).Str("filename", imageFilename).Msg("Sending schedule photo...")
	_, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:          chat.TgChatID,
		MessageThreadID: messageThreadID,
		Photo:           &tgmodels.InputFileUpload{Filename: imageFilename, Data: bytes.NewReader(imageData)},
		ReplyMarkup:     replyMarkup,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send schedule photo")
		err2 := SendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID: chat.TgChatID,
			Text:   ErrMsgCouldNotSendSchedule,
		})
		return errors.Join(err, err2)
	}
	return err
}

func WeekScheduleMarkup(conf model.ScheduleConfig) tgmodels.ReplyMarkup {
	var button tgmodels.InlineKeyboardButton
	if conf.Group != nil {
		button = UpdateInlineButton("group", string(conf.Group.GroupName))
	} else if conf.Teacher != nil {
		button = UpdateInlineButton("teacher", conf.Teacher.TeacherID.String())
	} else {
		return nil
	}
	markup := tgmodels.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgmodels.InlineKeyboardButton{{button}},
	}
	return markup
}

func SendErrorMessage(ctx context.Context, b *bot.Bot, params *bot.SendMessageParams) error {
	err := bothelper.SendTempMessage(ctx, b, 7*time.Second, params)
	if err != nil {
		log.Error().Err(err).Any("params", params).Msg("Failed to send error message")
	}
	return err
}

func UpdateInlineButton(kind, value string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{
		Text: "Обновить",
		CallbackData: fmt.Sprintf("update_%s\n%s\n%s",
			kind, value,
			time.Now().Format("20060102150405000"), // NOTE: Time is added to prevent editing message error when the content is the same.
		),
	}
}

var ErrNoAvailableProxy = errors.New("no available proxy")
var ErrEmptyProxy = errors.New("empty proxy")
var ErrProxyUnavailable = errors.New("proxy unavailable")

func FindAvailableProxy() (string, error) {
	proxies, err := loadProxies()
	if err != nil {
		return "", fmt.Errorf("failed to load proxies: %w", err)
	}

	result, ok := findFirstAsync(proxies, func(p string) bool { return check(p) == nil })
	if ok {
		log.Debug().Str("proxy", result).Msg("Found available proxy")
		return result, nil
	}
	log.Debug().Msg("No available proxy found")
	return "", ErrNoAvailableProxy
}

func check(proxy string) error {
	if proxy == "" {
		return ErrEmptyProxy
	}

	// Check the proxy by trying to launch Telegram bot polling with fake token.
	httpClient, err := NewHTTPProxyClient(proxy)
	if err != nil {
		// Proxy is not available
		return fmt.Errorf("%w: %w", ErrProxyUnavailable, err)
	}
	opts := []bot.Option{bot.WithHTTPClient(30*time.Second, httpClient)}
	_, err = bot.New("faketoken", opts...)
	if strings.Contains(err.Error(), "not found") {
		// Telegram servers are abailable, the proxy is available
		return nil
	}
	// Telegram servers are unavailable, the proxy is unavailable
	return ErrProxyUnavailable
}

func NewHTTPProxyClient(p string) (*http.Client, error) {
	dialer, err := proxy.SOCKS5("tcp", p, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}
	httpTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	httpClient := &http.Client{Transport: httpTransport}
	return httpClient, nil
}

const proxyListFile = "./storage/proxies.json"

func loadProxies() ([]string, error) {
	bytes, err := os.ReadFile(proxyListFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load proxy list file: %w", err)
	}

	var proxies []struct {
		Protocol    string `json:"protocol"`
		IP          string `json:"ip"`
		Port        int    `json:"port"`
		Geolocation struct {
			Country string `json:"country"`
		} `json:"geolocation"`
	}
	json.Unmarshal(bytes, &proxies)

	// Filter SOCKS and not Russian proxies
	var filteredProxies []string
	for _, proxy := range proxies {
		if proxy.Protocol == "socks5" && proxy.Geolocation.Country != "RU" {
			filteredProxies = append(filteredProxies, fmt.Sprintf("%s:%d", proxy.IP, proxy.Port))
		}
	}
	return filteredProxies, nil
}

func findFirstAsync[T any](items []T, predicate func(T) bool) (T, bool) {
	resultChan := make(chan T, 1)
	var wg sync.WaitGroup

	for _, item := range items {
		wg.Add(1)
		go func(item T) {
			defer wg.Done()
			if predicate(item) {
				select {
				case resultChan <- item:
				default:
				}
			}
		}(item)
	}

	// Wait for all goroutines to finish in a separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	result, ok := <-resultChan
	return result, ok // ok=false means no match found
}

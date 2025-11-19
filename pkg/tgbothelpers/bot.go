package tgbothelpers

import (
	"context"
	"fmt"
	"regexp"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
)

type BotAPIProvider interface {
	API() *tgbotapi.BotAPI
}

type UpdateHandler interface {
	OnUpdate(tgbotapi.Update)
}

func StartPolling(bot interface {
	BotAPIProvider
	UpdateHandler
}) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.API().GetUpdatesChan(u)
	for update := range updates {
		go bot.OnUpdate(update)
	}
}

// Deprecated, use SetMyCommands instead
func SetMyCommandsOld(api *tgbotapi.BotAPI, commandsData []map[string]string) {
	log.Trace().Any("commands", commandsData).Msg("Setting my commands")
	commands := make([]tgbotapi.BotCommand, len(commandsData))
	for i, cmd := range commandsData {
		for name, desc := range cmd {
			commands[i] = tgbotapi.BotCommand{Command: name, Description: desc}
		}
	}
	api.Send(tgbotapi.NewSetMyCommands(commands...))
}

func SetMyCommands(ctx context.Context, b *bot.Bot, commandsData []map[string]string) (bool, error) {
	log.Trace().Any("commands", commandsData).Msg("Setting my commands")
	commands := []models.BotCommand{}
	for _, cmd := range commandsData {
		for name, desc := range cmd {
			commands = append(commands, models.BotCommand{
				Command:     name,
				Description: desc,
			})
		}
	}
	return b.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: commands})
}

// Deprecated, use SendTempMessage instead
func SendTempMessageOld(api *tgbotapi.BotAPI, tgChatID int64, text string, dur time.Duration) error {
	newMsg := tgbotapi.NewMessage(tgChatID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	sentMsg, err := api.Send(newMsg)

	go func() {
		time.Sleep(dur)
		api.Send(tgbotapi.NewDeleteMessage(tgChatID, sentMsg.MessageID))
	}()

	return err
}

func SendTempMessage(ctx context.Context, b *bot.Bot, dur time.Duration, params *bot.SendMessageParams) error {
	msg, err := b.SendMessage(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("Error sending temporary message")
		return fmt.Errorf("error sending temporary message: %w", err)
	}

	go func() {
		time.Sleep(dur)
		success, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    msg.Chat.ID,
			MessageID: msg.ID,
		})
		if err != nil {
			log.Error().Err(err).Msg("Error deleting temporary message")
		}
		if !success {
			log.Error().Msg("Failed to delete temporary message")
		}
	}()

	return nil
}

func ParseCommand(text string) (command string, args string) {
	re := regexp.MustCompile(`^/([a-zA-Z0-9_]+)\s+(.+)$`)
	matches := re.FindStringSubmatch(text)
	if len(matches) == 3 {
		command = matches[1]
		args = matches[2]
	} else {
		command = text
	}
	return command, args
}

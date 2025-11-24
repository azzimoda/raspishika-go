package tgbothelpers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
)

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

// DeleteMessageSafely deletes a message safely, checking if the bot has permission to do it in group chats.
func DeleteMessageSafely(ctx context.Context, b *bot.Bot, message *models.Message) (bool, error) {
	if message.Chat.Type == models.ChatTypePrivate {
		return b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: message.Chat.ID, MessageID: message.ID})
	}

	// Check if bot can delete messages in group.
	me, err := b.GetMe(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get bot info: %w", err)
	}

	botMember, err := b.GetChatMember(ctx, &bot.GetChatMemberParams{ChatID: message.Chat.ID, UserID: me.ID})
	if err != nil {
		return false, fmt.Errorf("failed to check bot permissions: %w", err)
	}

	if botMember.Administrator != nil && botMember.Administrator.CanDeleteMessages {
		return b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: message.Chat.ID, MessageID: message.ID})
	}

	return false, nil
}

func ParseCommand(text string) (command string, args string) {
	re := regexp.MustCompile(`^/(\w+)(@\w+)?(\s([\s\S]+))?$`)
	submatches := re.FindStringSubmatch(text)
	if submatches != nil {
		command = submatches[1]
		args = submatches[4]
	} else {
		command = text
	}
	return command, args
}

func ParseCallbackData(data string) CallbackCommand {
	lines := strings.Split(data, "\n")
	command := ""
	if len(lines) >= 1 {
		command = lines[0]
	}
	args := []string{}
	if len(lines) >= 2 {
		args = lines[1:]
	}
	return CallbackCommand{Command: command, Args: args}
}

type CallbackCommand struct {
	Command string
	Args    []string
}

func (cc CallbackCommand) Arg(i int) string {
	if i < len(cc.Args) {
		return cc.Args[i]
	}
	return ""
}

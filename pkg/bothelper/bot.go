package bothelper

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

// SendTempMessage sends a temporary message that will be automatically deleted after the specified duration.
func SendTempMessage(ctx context.Context, b *bot.Bot, dur time.Duration, params *bot.SendMessageParams) error {
	msg, err := b.SendMessage(ctx, params)
	if err != nil {
		log.Error().Err(err).Msg("Error sending temporary message")
		return fmt.Errorf("error sending temporary message: %w", err)
	}

	go func() {
		time.Sleep(dur)
		success, err := DeleteMessageSafely(ctx, b, msg)
		if err != nil {
			log.Error().Err(err).Msg("Error deleting temporary message")
		}
		if !success {
			log.Warn().Msg("Temporary message could not be deleted")
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

	log.Warn().Msg("Bot does not have permission to delete messages in this group")
	return false, nil
}

// ParseCommand parses a command string into a command and arguments.
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

// ParseCallbackData parses callback data into a CallbackCommand struct.
//
// The data is expected to be in the format of a command followed by newline-separated arguments.
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

// CallbackCommand represents a parsed callback command with a command and arguments.
type CallbackCommand struct {
	Command string
	Args    []string
}

// Arg returns the argument at the specified index, or an empty string if out of bounds.
func (c CallbackCommand) Arg(i int) string {
	if i < len(c.Args) {
		return c.Args[i]
	}
	return ""
}

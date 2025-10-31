package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

// onStart handles /start command. It greets the user in a very friendly way ;)
func (b *AdminBot) onStart(msg *tgbotapi.Message) error {
	_, err := b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "Welcome back, Master!"))
	return err
}

func (b *AdminBot) onChat(msg *tgbotapi.Message) error {

	chatID, err := strconv.ParseInt(msg.CommandArguments(), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse chat ID: %w", err)
	}

	return b.sendChatReport(chatID, msg)
}

func (b *AdminBot) sendChatReport(chatID int64, msg *tgbotapi.Message) error {
	chat, err := b.repo.GetChatByChatID(chatID)
	if err != nil {
		return fmt.Errorf("failed to get chat by chat ID: %w", err)
	}

	text := b.chatReport(b.repo, chat)
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err = b.api.Send(newMsg)
	return err
}

func (b *AdminBot) chatReport(repo *database.Repository, chat *database.Chat) string {
	recentTeachers, err := repo.GetTeacherByChatID(chat.ID)
	if err != nil {
		recentTeachers = []database.Teacher{}
		b.Report().Err(err).Sendf("Failed to get recent teachers for chat %d", chat.ID)
	}
	recentTeachersNames := make([]string, len(recentTeachers))
	for i, t := range recentTeachers {
		recentTeachersNames[i] = t.Name
	}

	return fmt.Sprintf(
		"ID: %d\n"+
			"Chat ID: `%d`\n"+
			"Username: %s\n"+
			"State: %s\n"+
			"Department: %s\n"+
			"Group: `%s`\n"+
			"Daily Sending Time: %s\n"+
			"Pair Sending: %t\n"+
			"Access: %d\n\n"+
			"Recent Teachers: %s",
		chat.ID,
		chat.ChatID,
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, utils.DerefOrTypeDefault(chat.UserName)),
		chat.State,
		utils.DerefOrTypeDefault(chat.DepartmentName),
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, utils.DerefOrTypeDefault(chat.GroupName)),
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, chat.DailySendingTime),
		chat.PairSending,
		chat.Access,
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, strings.Join(recentTeachersNames, ", ")),
	)
}

func (b *AdminBot) onGroup(msg *tgbotapi.Message) error {
	group := msg.CommandArguments()

	group, err := utils.ValidateGroupNameFormat(group)
	if err != nil {
		return fmt.Errorf("failed to validate group name format: %w", err)
	}

	return b.sendGroupReport(group, msg)
}

func (b *AdminBot) sendGroupReport(group string, msg *tgbotapi.Message) error {
	group, err := b.repo.ValidateGroupNameCase(group)
	if err != nil {
		return fmt.Errorf("failed to validate group name case: %w", err)
	}

	chats, err := b.repo.GetChatsByGroup(group)
	if err != nil {
		return fmt.Errorf("failed to get chats by group: %w", err)
	}

	if len(chats) == 0 {
		return fmt.Errorf("no chats found for group %s", group)
	}

	text := tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, fmt.Sprintf("Chats in group `%s` (%d):\n", group, len(chats)))
	for _, chat := range chats {
		text += fmt.Sprintf("• `/chat %d`\n", chat.ChatID)
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err = b.api.Send(newMsg)
	return err
}

func (b *AdminBot) onChats(msg *tgbotapi.Message) error {
	chats, err := b.repo.GetChats()
	if err != nil {
		return fmt.Errorf("failed to get chats: %w", err)
	}

	totalCount := len(chats)
	inactiveCount := 0
	for _, chat := range chats {
		// Chat is inactive if it didn't use any commands for 48 hours and have disabled all sendings.
		recentCommandUsages, err := b.repo.GetRecentChatUpdateLogs(chat.ID, 48*time.Hour)
		if err != nil {
			b.Report().Err(err).Sendf("Failed to get recent command usages for chat %d", chat.ID)
			continue
		}
		if len(recentCommandUsages) == 0 {
			inactiveCount += 1
		}
	}

	text := fmt.Sprintf("Total chats: %d\nActive chats: %d\nInactive chats: %d", totalCount, totalCount-inactiveCount, inactiveCount)
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err = b.api.Send(newMsg)
	return err
}

package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

// onStart handles /start command. It greets the user in a very friendly way ;)
func (b *AdminBot) onStart(msg *tgbotapi.Message) error {
	_, err := b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "Welcome back, Master!"))
	return err
}

func (b *AdminBot) onChat(msg *tgbotapi.Message) error {
	tgChatID, err := strconv.ParseInt(msg.CommandArguments(), 10, 64)
	var chat *database.Chat
	if err != nil {
		// Get last chat if no chat ID provided.
		chats, err := b.repo.GetChats()
		if err != nil {
			return fmt.Errorf("failed to get chats: %w", err)
		}
		if len(chats) == 0 {
			return fmt.Errorf("no chats found")
		}

		chat = &chats[len(chats)-1]
	} else {
		chat, err = b.repo.GetChatByTgChatID(tgChatID)
		if err != nil {
			return fmt.Errorf("failed to get chat by chat ID: %w", err)
		}
	}

	return b.sendChatReport(chat, msg)
}

func (b *AdminBot) sendChatReport(chat *database.Chat, msg *tgbotapi.Message) error {
	text := b.chatReport(b.repo, chat)
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err := b.api.Send(newMsg)
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

	recentUpdates, err := repo.GetRecentChatUpdateLogs(chat.ID, 48*time.Hour)
	if err != nil {
		log.Error().Err(err).Int("chat.ID", chat.ID).Msg("Failed to get recent updates for chat")
		recentUpdates = []database.UpdateLog{}
	}
	recentUpdatesStr := ""
	for i := len(recentUpdates) - 1; i >= 0 && i >= len(recentUpdates)-5; i-- {
		recentUpdatesStr += tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, fmt.Sprintf("- %s (time: %dms, error: %s)\n",
			recentUpdates[i].Data, recentUpdates[i].HandlingTime, utils.DerefOrTypeDefault(recentUpdates[i].Error)))
	}

	return fmt.Sprintf(
		`ID: %d
Chat ID: %s
Username: @%s
State: %s
Department: %s
Group: %s
Daily Sending Time: %s
Pair Sending: %t
Access: %d

Recent Teachers: %s

Recent updates:
%s`,
		chat.ID,
		fmt.Sprintf("`%d`", chat.TgChatID),
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, utils.DerefOrTypeDefault(chat.UserName)),
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, string(chat.State)),
		utils.DerefOrTypeDefault(chat.DepartmentName),
		"`"+tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, utils.DerefOrTypeDefault(chat.GroupName))+"`",
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, utils.DerefOrTypeDefault(chat.DailySendingTime)),
		chat.PairSending,
		chat.Access,
		tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, strings.Join(recentTeachersNames, ", ")),
		recentUpdatesStr,
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
		text += fmt.Sprintf("• `/chat %d`\n", chat.TgChatID)
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

func (b *AdminBot) onUpdates(msg *tgbotapi.Message) error {
	startTime := time.Now().Add(-24 * time.Hour) // TODO: Make it configurable from args.

	updateLogs, err := b.repo.GetUpdateLogsByPeriod(startTime, time.Now())
	if err != nil {
		return fmt.Errorf("failed to get update logs by period: %w", err)
	}

	errorCount := 0
	scheduleCommandCount := 0
	updateCallbackCount := 0
	for _, log := range updateLogs {
		if log.Error != nil && *log.Error != "" {
			errorCount += 1
		}

		if log.Kind == "message" &&
			(log.Data == "/week" || log.Data == "Неделя" || log.Data == "/tomorrow" || log.Data == "Завтра" ||
				log.Data == "/left" || log.Data == "Сегодня") {
			scheduleCommandCount += 1
		}

		if log.Kind == "callback_query" && strings.Contains(log.Data, "update_") {
			updateCallbackCount += 1
		}
	}

	successfulCount := len(updateLogs) - errorCount
	newMsg := tgbotapi.NewMessage(
		msg.Chat.ID,
		fmt.Sprintf("Success: %d\nError: %d\nSchedule: %d\nUpdate callback: %d",
			successfulCount, errorCount, scheduleCommandCount, updateCallbackCount),
	)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err = b.api.Send(newMsg)
	return err
}

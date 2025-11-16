package bot

import (
	"errors"
	"fmt"
	"sort"
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
	log.Debug().Msg("AdminBot.onChats()")

	log.Trace().Msg("Counting...")
	totalChats, err := b.repo.GetChatCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get total chat count")
		totalChats = 0
	}
	log.Trace().Int("totalChats", totalChats).Send()

	log.Trace().Msg("Counting private and group chats...")
	privateChatCount, err := b.repo.GetPrivateChatCount()
	groupChatCount := totalChats - privateChatCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get private chat count")
		privateChatCount = 0
		groupChatCount = 0
	}

	log.Trace().Msg("Counting active and inactive chats...")
	inactiveCount, err := b.repo.GetInactiveChatCount(48 * time.Hour) // TODO: Make duration configurable.
	activeChatCount := totalChats - inactiveCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get inactive chat count")
		inactiveCount = 0
		activeChatCount = 0
	}

	// General info.
	text := fmt.Sprintf(`Total chats: %d
Private chats: %d
Group chats: %d
Active chats: %d
Inactive chats: %d`,
		totalChats, privateChatCount, groupChatCount, activeChatCount, inactiveCount)
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err1 := b.api.Send(newMsg)

	err2 := b.sendConfigReport(msg)

	return errors.Join(err1, err2)
}

func (b *AdminBot) sendConfigReport(msg *tgbotapi.Message) error {
	log.Trace().Msg("AdminBot.sendConfigReport()")

	log.Trace().Msg("Getting daily sending time counts...")
	dailyTimes, err := b.repo.GetChatGroupedByDailySendingTime()
	if err != nil {
		return fmt.Errorf("failed to get chats grouped by daily sending time: %w", err)
	}

	log.Trace().Msg("Getting daily sending time enabled counts...")
	dailyEnabledCount, err := b.repo.GetChatCountWithDailySendingEnabled()
	if err != nil {
		return fmt.Errorf("failed to get chats with daily sending time enabled: %w", err)
	}

	log.Trace().Msg("Getting pair sending enabled counts...")
	pairEnabledCount, err := b.repo.GetChatCountWithPairSendingEnabled()
	if err != nil {
		return fmt.Errorf("failed to get chats with pair sending enabled: %w", err)
	}

	timeKeys := make([]string, 0, len(dailyTimes))
	for k := range dailyTimes {
		timeKeys = append(timeKeys, k)
	}
	sort.Strings(timeKeys)

	text := fmt.Sprintf("Pair enabled: %d\nDaily enabled: %d\nTimes:\n```\n", pairEnabledCount, dailyEnabledCount)
	for _, t := range timeKeys {
		text += fmt.Sprintf("\\- %s: %3d\n", t, dailyTimes[t])
	}
	text += "```\n"

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

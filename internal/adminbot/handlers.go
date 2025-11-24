package adminbot

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/tgbothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (ab *AdminBot) registerHandlers() {
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommandStartOnly, ab.startHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "chat", bot.MatchTypeCommand, ab.chatHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "group", bot.MatchTypeCommand, ab.groupHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "chats", bot.MatchTypeCommand, ab.chatsHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "updates", bot.MatchTypeCommand, ab.updatesHandler)
}

func (ab *AdminBot) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Debug().Msgf("Received update: %v", update)

	if update.Message != nil {
		if strings.HasPrefix(update.Message.Text, "/") {
			// Remove not handled command.
			b.DeleteMessage(ctx, &bot.DeleteMessageParams{ChatID: update.Message.Chat.ID, MessageID: update.Message.ID})
		}
	}
}

func (ab *AdminBot) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Welcome back, Master!",
	})
}

func (ab *AdminBot) chatHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	_, args := tgbothelpers.ParseCommand(update.Message.Text)

	tgChatID, err := strconv.ParseInt(args, 10, 64)
	var chat *database.Chat
	if err != nil {
		// Get last chat from database.
		chats, err := ab.services.Repo.GetChats()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get chats")
			return
		}
		if len(chats) == 0 {
			log.Error().Msg("No chats found")
			return
		}

		chat = &chats[len(chats)-1]
	} else {
		chat, err = ab.services.Repo.GetChatByTgChatID(tgChatID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get chat by chat ID")
			return
		}
	}

	ab.sendChatReport(chat, b, ctx, update)
}

func (ab *AdminBot) sendChatReport(chat *database.Chat, b *bot.Bot, ctx context.Context, update *models.Update) {
	recentTeachers, err := ab.services.Repo.GetTeacherByChatID(chat.ID)
	if err != nil {
		recentTeachers = []database.Teacher{}
		ab.services.Reporter.Report().Err(err).Msgf("Failed to get recent teachers for chat %d", chat.ID)
	}
	recentTeachersNames := make([]string, len(recentTeachers))
	for i, t := range recentTeachers {
		recentTeachersNames[i] = t.Name
	}

	recentUpdates, err := ab.services.Repo.GetRecentChatUpdateLogs(chat.ID, 48*time.Hour)
	if err != nil {
		log.Error().Err(err).Int("chat.ID", chat.ID).Msg("Failed to get recent updates for chat")
		recentUpdates = []database.UpdateLog{}
	}
	recentUpdatesStr := ""
	for i := len(recentUpdates) - 1; i >= 0 && i >= len(recentUpdates)-5; i-- {
		recentUpdatesStr += bot.EscapeMarkdown(fmt.Sprintf("- %s (time: %dms, error: %s)\n",
			recentUpdates[i].Data, recentUpdates[i].HandlingTime, utils.DerefOrTypeDefault(recentUpdates[i].Error)))
	}

	text := fmt.Sprintf(
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
		bot.EscapeMarkdown(utils.DerefOrTypeDefault(chat.UserName)),
		bot.EscapeMarkdown(string(chat.State)),
		utils.DerefOrTypeDefault(chat.DepartmentName),
		"`"+bot.EscapeMarkdown(utils.DerefOrTypeDefault(chat.GroupName))+"`",
		bot.EscapeMarkdown(utils.DerefOrTypeDefault(chat.DailySendingTime)),
		chat.PairSending,
		chat.Access,
		bot.EscapeMarkdown(strings.Join(recentTeachersNames, ", ")),
		recentUpdatesStr,
	)

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text})
}

func (ab *AdminBot) groupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	_, group := tgbothelpers.ParseCommand(update.Message.Text)

	group, err := utils.ValidateGroupNameFormat(group)
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate group name format")
		return
	}

	chats, err := ab.services.Repo.GetChatsByGroup(group)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats by group")
		return
	}

	if len(chats) == 0 {
		log.Error().Msgf("No chats found for group %s", group)
		return
	}

	text := bot.EscapeMarkdown(fmt.Sprintf("Chats in group `%s` (%d):\n", group, len(chats)))
	for _, chat := range chats {
		text += fmt.Sprintf("• `/chat %d`\n", chat.TgChatID)
	}

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text})
}

func (ab *AdminBot) chatsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	totalChats, err := ab.services.Repo.GetChatCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get total chat count")
		totalChats = 0
	}

	privateChatCount, err := ab.services.Repo.GetPrivateChatCount()
	groupChatCount := totalChats - privateChatCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get private chat count")
		privateChatCount = 0
		groupChatCount = 0
	}

	inactiveCount, err := ab.services.Repo.GetInactiveChatCount(48 * time.Hour) // TODO: Make duration configurable.
	activeChatCount := totalChats - inactiveCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get inactive chat count")
		inactiveCount = 0
		activeChatCount = 0
	}

	text := fmt.Sprintf(`Total chats: %d
Private chats: %d
Group chats: %d
Active chats: %d
Inactive chats: %d`,
		totalChats, privateChatCount, groupChatCount, activeChatCount, inactiveCount)
	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text})

	ab.sendConfigReport(ctx, b, update)
}

func (ab *AdminBot) sendConfigReport(ctx context.Context, b *bot.Bot, update *models.Update) {
	dailyTimes, err := ab.services.Repo.GetChatGroupedByDailySendingTime()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats grouped by daily sending time")
		return
	}

	dailyEnabledCount, err := ab.services.Repo.GetChatCountWithDailySendingEnabled()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with daily sending time enabled")
		return
	}

	pairEnabledCount, err := ab.services.Repo.GetChatCountWithPairSendingEnabled()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with pair sending enabled")
		return
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

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text})
}

func (ab *AdminBot) updatesHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	startTime := time.Now().Add(-24 * time.Hour) // TODO: Make it configurable from args.

	updateLogs, err := ab.services.Repo.GetUpdateLogsByPeriod(startTime, time.Now())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get update logs by period")
		return
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

	text := fmt.Sprintf("Success: %d\nError: %d\nSchedule: %d\nUpdate callback: %d",
		successfulCount, errorCount, scheduleCommandCount, updateCallbackCount)
	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text})
}

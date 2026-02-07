package adminbot

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/repository"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (ab *AdminBot) registerHandlers() {
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommandStartOnly, ab.startHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "chat", bot.MatchTypeCommand, ab.chatHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "group", bot.MatchTypeCommand, ab.groupHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "config", bot.MatchTypeCommand, ab.configHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "/dist_", bot.MatchTypePrefix, ab.distHandler)

	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "stats", bot.MatchTypeCommand, ab.statsHandler)
}

func (ab *AdminBot) defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Debug().Msgf("Received update: %v", update)

	if update.Message != nil {
		if strings.HasPrefix(update.Message.Text, "/") {
			// Remove not handled command.
			bothelpers.DeleteMessageSafely(ctx, b, update.Message)
		}

		if _, err := ab.services.Repo.ValidateGroupName(update.Message.Text); err == nil {
			ab.groupHandler(ctx, b, update)
			return
		}

		// TODO: Handle username and chat_id.
	}
}

func (ab *AdminBot) startHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Welcome back, Master!",
	})
}

func (ab *AdminBot) chatHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	_, args := bothelpers.ParseCommand(update.Message.Text)

	tgChatID, err := strconv.ParseInt(args, 10, 64)
	var chat *repository.Chat
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

func (ab *AdminBot) sendChatReport(chat *repository.Chat, b *bot.Bot, ctx context.Context, update *models.Update) {
	recentTeachers, err := ab.services.Repo.GetTeacherByChatID(chat.ID)
	if err != nil {
		recentTeachers = []repository.Teacher{}
		ab.services.Reporter.Report().Err(err).Msgf("Failed to get recent teachers for chat %d", chat.ID)
	}
	recentTeachersNames := make([]string, len(recentTeachers))
	for i, t := range recentTeachers {
		recentTeachersNames[i] = t.Name
	}

	recentUpdates, err := ab.services.Repo.GetRecentChatUpdateLogs(chat.ID, 48*time.Hour)
	if err != nil {
		log.Error().Err(err).Int("chat.ID", chat.ID).Msg("Failed to get recent updates for chat")
		recentUpdates = []repository.UpdateLog{}
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

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
}

func (ab *AdminBot) groupHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	group := update.Message.Text // For default handler
	if strings.HasPrefix(update.Message.Text, "/") {
		_, group = bothelpers.ParseCommand(update.Message.Text) // For /group command
	}

	group, err := ab.services.Repo.ValidateGroupName(update.Message.Text)
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

	var text strings.Builder
	text.WriteString(bot.EscapeMarkdown(fmt.Sprintf("Chats in group `%s` (%d chats):\n", group, len(chats))))
	for _, chat := range chats {
		fmt.Fprintf(&text, "• `/chat %d`\n", chat.TgChatID)
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text.String(),
		ParseMode: models.ParseModeMarkdown,
	})
}

func (ab *AdminBot) configHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
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

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
}

func (ab *AdminBot) statsHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	_, args := bothelpers.ParseCommand(update.Message.Text)
	duration, ok := parsePeriod(args)
	if !ok {
		duration = 24 * time.Hour
	}

	totalChats, err := ab.services.Repo.GetChatCount()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chat count")
		return
	}

	privateChatCount, err := ab.services.Repo.GetPrivateChatCount()
	groupChatCount := totalChats - privateChatCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get private chat count")
		privateChatCount = 0
		groupChatCount = 0
	}

	inactiveCount, err := ab.services.Repo.GetInactiveChatCount(duration)
	activeChatCount := totalChats - inactiveCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get inactive chat count")
		inactiveCount = 0
		activeChatCount = 0
	}

	newChats, err := ab.services.Repo.GetNewChatCount(duration)

	updateLogs, err := ab.services.Repo.GetUpdateLogsByPeriod(time.Now().Add(-duration), time.Now())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get update logs by period")
		return
	}

	totalUpdates := len(updateLogs)
	errorCount := 0
	scheduleCommandCount := 0
	callbackCount := 0
	for _, log := range updateLogs {
		if log.Error != nil && *log.Error != "" {
			errorCount += 1
		}

		if log.Kind == "message" &&
			(log.Data == "/week" || log.Data == "Неделя" ||
				log.Data == "/tomorrow" || log.Data == "Завтра" ||
				log.Data == "/left" || log.Data == "Сегодня") {
			scheduleCommandCount += 1
		}

		if log.Kind == "callback_query" && strings.Contains(log.Data, "update_") {
			callbackCount += 1
		}
	}

	// TODO: Collect metrics...

	text := fmt.Sprintf(
		`MONTHLY STATISTICS

Total: %d
Private/Group: %d / %d
Active/Inactive: %d / %d
New reigstered: %d

Updates: %d
Success/Fail: %d / %d
Schedule: %d
Callbacks: %d`,
		totalChats,
		privateChatCount, groupChatCount,
		activeChatCount, inactiveCount,
		newChats,
		totalUpdates,
		totalUpdates-errorCount, errorCount,
		scheduleCommandCount, callbackCount,
	)

	b.SendMessage(ctx, &bot.SendMessageParams{ChatID: update.Message.Chat.ID, Text: text})
}

func (ab *AdminBot) distHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	log.Trace().Msg("distHandler")
	command, args := bothelpers.ParseCommand(update.Message.Text)

	log.Trace().Msg("Parsing parameters...")
	dataKind := "a"   // Variants: a, ...
	distPeriod := "w" // Variants: w - week days, h - hours of day
	parts := strings.Split(command, "_")
	if len(parts) == 2 {
		suffix := parts[1]
		if len(suffix) >= 1 {
			switch suffix[0] {
			case 'a':
				dataKind = string(suffix[0])
			}
		}
		if len(suffix) >= 2 {
			switch suffix[1] {
			case 'w', 'h':
				distPeriod = string(suffix[1])
			}
		}
	}
	log.Trace().Str("dataKind", dataKind).Str("distPeriod", distPeriod).Send()

	log.Trace().Msg("Parsing period parameter...")
	dur, ok := parsePeriod(args)
	if !ok {
		dur = 30 * 24 * time.Hour // Default: month
	}
	log.Trace().Dur("dur", dur).Send()

	log.Trace().Msg("Fetching distribution...")
	distribution, err := ab.services.Repo.GetDist(dataKind, distPeriod, dur)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get distribution")
		return
	}
	log.Trace().Int("len(distribution)", len(distribution)).Send()

	log.Trace().Msg("Building message text...")
	var text strings.Builder
	text.WriteString("```\n")
	for _, s := range distribution {
		fmt.Fprintf(&text, "%s: %d\n", s.Name, s.Value)
	}
	text.WriteString("\n```")

	log.Trace().Msg("Sending message...")
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text.String(),
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		log.Error().Err(err).Send()
	}
	log.Trace().Msg("distHandler: Done!")
}

func parsePeriod(str string) (time.Duration, bool) {
	if str == "" {
		return 0, false
	}

	re := regexp.MustCompile(`^(\d+)\s*(h|d|w|m|y)?$`)
	matches := re.FindStringSubmatch(str)
	multiplier := time.Hour
	switch matches[2] {
	// case "h":
	// 	multiplier = time.Hour
	case "d":
		multiplier = 24 * time.Hour
	case "w":
		multiplier = 7 * 24 * time.Hour
	case "m":
		multiplier = 30 * 24 * time.Hour
	case "y":
		multiplier = 365 * 24 * time.Hour
		// Defualt is hours
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return time.Duration(multiplier * time.Duration(num)), true
}

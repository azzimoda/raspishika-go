package adminbot

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

var scheduleCommandRe = regexp.MustCompile(`^((/week|/tomorrow|/left)(@\w+)?|Неделя|Завтра|Сегодня)$`)

func (ab *AdminBot) statsHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	_, args := bothelpers.ParseCommand(update.Message.Text)
	duration, ok := parsePeriod(args)
	if !ok {
		duration = 24 * time.Hour
	}

	// Chats data
	totalChats, err := models.GetChatCount(ab.services.Repo.DB)
	if err != nil {
		ab.Report().Log().Err(err).Msg("Failed to get chat count")
		return
	}
	privateChatCount, err := models.GetPrivateChatCount(ab.services.Repo.DB)
	groupChatCount := totalChats - privateChatCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get private chat count")
		privateChatCount = -1
		groupChatCount = -1
	}
	inactiveCount, err := models.GetInactiveChatCount(ab.services.Repo.DB, duration)
	activeCount := totalChats - inactiveCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get inactive chat count")
		inactiveCount = -1
		activeCount = -1
	}
	chatsNewCount, err := models.GetNewChatCount(ab.services.Repo.DB, duration)
	if err != nil {
		ab.Report().Log().Err(err).Msg("Failed to get new chats count")
		return
	}

	chatsNewGrouped, err := models.GetNewChatsGrouped(ab.services.Repo.DB, duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get new chats grouped")
	}

	// Groups data
	groupCount, err := models.GetConfiguredGroupCount(ab.services.Repo.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get configured group count")
		groupCount = -1
	}
	chatsPerGroupAvg, err := models.GetAvgChatsPerGroup(ab.services.Repo.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get average chats per group")
		chatsPerGroupAvg = -1
	}
	chatsPerGroupMedian, err := models.GetMedianChatsPerGroup(ab.services.Repo.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get median chats per group")
		chatsPerGroupMedian = -1
	}

	// Updates data
	updateLogs, err := models.GetUpdateLogsByPeriod(ab.services.Repo.DB, time.Now().Add(-duration), time.Now())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get update logs by period")
		return
	}
	totalUpdates := len(updateLogs)
	errorCount := 0
	scheduleCommandCount := 0
	callbackCount := 0
	// TODO: Move this logic into SQL.
	for _, log := range updateLogs {
		if log.Error != nil && *log.Error != "" {
			errorCount += 1
		}

		if log.Kind == "message" && scheduleCommandRe.MatchString(log.Data) ||
			log.Kind == "callback_query" && strings.Contains(log.Data, "teacher") {

			scheduleCommandCount += 1
		}

		if log.Kind == "callback_query" && strings.Contains(log.Data, "update_") {
			callbackCount += 1
		}
	}

	// Sendings data
	totalSendings, sendingOkCount, sendingFailCount, err := models.GetSendingLogsCount(ab.services.Repo.DB, models.SendingLogAny, duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get total sending logs")
	}
	dailySendings, _, _, err := models.GetSendingLogsCount(ab.services.Repo.DB, models.SendingLogDaily, duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get daily sending logs")
	}
	pairSendings, _, _, err := models.GetSendingLogsCount(ab.services.Repo.DB, models.SendingLogPair, duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get pair sending logs")
	}
	updateSendings, _, _, err := models.GetSendingLogsCount(ab.services.Repo.DB, models.SendingLogUpdate, duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get update sending logs")
	}

	// Format and send message
	text := generalReport{
		period:       duration,
		chatsTotal:   totalChats,
		chatsPrivate: privateChatCount, chatsGroup: groupChatCount,
		chatsActive: activeCount, chatsInactive: inactiveCount,
		chatsNew: chatsNewCount, chatsNewGrouped: chatsNewGrouped,

		groupsTotal: groupCount, chatsPerGroupAvg: chatsPerGroupAvg, chatsPerGroupMedian: chatsPerGroupMedian,

		updatesTotal:   totalUpdates,
		updatesSuccess: totalUpdates - errorCount, updatesFail: errorCount,
		updatesSchedule: scheduleCommandCount, updatesCallback: callbackCount,

		sendingsTotal: totalSendings,
		sendingsDaily: dailySendings, sendingsPair: pairSendings, sendingsUpdate: updateSendings,
		sendingsSuccess: sendingOkCount, sendingsFail: sendingFailCount,
	}.HTML()

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("Failed to send message: %v", err),
		})
	}
}

type generalReport struct {
	period time.Duration

	chatsTotal      int
	chatsPrivate    int
	chatsGroup      int
	chatsActive     int
	chatsInactive   int
	chatsNew        int
	chatsNewGrouped map[string]int // Group -> Count

	groupsTotal         int
	chatsPerGroupAvg    float32
	chatsPerGroupMedian float32

	updatesTotal    int
	updatesSuccess  int
	updatesFail     int
	updatesSchedule int
	updatesCallback int

	sendingsTotal   int
	sendingsDaily   int
	sendingsPair    int
	sendingsUpdate  int
	sendingsSuccess int
	sendingsFail    int
}

func (gr generalReport) HTML() string {
	var textNewChatsGrouped strings.Builder
	fmt.Fprint(&textNewChatsGrouped, "\n<pre>")
	for group, count := range gr.chatsNewGrouped {
		fmt.Fprintf(&textNewChatsGrouped, "- %s: %d\n", group, count)
	}
	fmt.Fprint(&textNewChatsGrouped, "</pre>")

	return fmt.Sprintf(`STATISTICS FOR LAST %s

Total: %d
Private/Group: %d / %d
Active/Inactive: %d / %d
New reigstered: %d
%s

Groups: %d
CpG: Avg/Median: %.2f / %.2f

Updates: %d
Success/Fail: %d / %d
Schedule/Callback: %d / %d

Sendings: %d
Daily/Pair/Change: %d / %d / %d
Success/Fail: %d / %d`,
		gr.period,
		gr.chatsTotal,
		gr.chatsPrivate, gr.chatsGroup,
		gr.chatsActive, gr.chatsInactive,
		gr.chatsNew,
		textNewChatsGrouped.String(),

		gr.groupsTotal,
		gr.chatsPerGroupAvg, gr.chatsPerGroupMedian,

		gr.updatesTotal,
		gr.updatesSuccess, gr.updatesFail,
		gr.updatesSchedule, gr.updatesCallback,

		gr.sendingsTotal,
		gr.sendingsDaily, gr.sendingsPair, gr.sendingsUpdate,
		gr.sendingsSuccess, gr.sendingsFail,
	)
}

func (ab *AdminBot) configHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	dailyTimes, err := models.GetChatGroupedByDailySendingTime(ab.services.Repo.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats grouped by daily sending time")
		return
	}

	dailyEnabledCount, err := models.GetChatCountWithDailySendingEnabled(ab.services.Repo.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with daily sending time enabled")
		return
	}

	pairEnabledCount, err := models.GetChatCountWithPairSendingEnabled(ab.services.Repo.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with pair sending enabled")
		return
	}

	updateEnabledCount, err := models.GetChatCountWithChangeAlertOn(ab.services.Repo.DB)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with update sending enabled")
		return
	}

	timeKeys := make([]string, 0, len(dailyTimes))
	for k := range dailyTimes {
		timeKeys = append(timeKeys, k)
	}
	sort.Strings(timeKeys)

	var text strings.Builder
	fmt.Fprintf(&text, "Pair enabled: %d\nDaily enabled: %d\nUpdate enabled: %d\nTimes:\n<pre>",
		pairEnabledCount, dailyEnabledCount, updateEnabledCount)
	for _, t := range timeKeys {
		fmt.Fprintf(&text, "- %s: %3d\n", t, dailyTimes[t])
	}
	text.WriteString("</pre>")

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text.String(),
		ParseMode: tgmodels.ParseModeHTML,
	})
}

func (ab *AdminBot) distHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
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
	distribution, err := models.GetDist(ab.services.Repo.DB, dataKind, distPeriod, dur)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get distribution")
		return
	}

	var text strings.Builder
	text.WriteString("<pre>")
	for _, s := range distribution {
		fmt.Fprintf(&text, "%s: %d\n", s.Name, s.Value)
	}
	text.WriteString("</pre>")

	log.Trace().Msg("Sending message...")
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text.String(),
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		log.Error().Err(err).Send()
	}
}

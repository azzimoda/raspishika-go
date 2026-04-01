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

	"github.com/azzimoda/raspishika-go/internal/model"
	bothelpers "github.com/azzimoda/raspishika-go/pkg/bothelper"
	"github.com/azzimoda/raspishika-go/pkg/refutil"
)

var scheduleCommandRe = regexp.MustCompile(`^((/week|/tomorrow|/left)(@\w+)?|Неделя|Завтра|Сегодня)$`)

func (ab *AdminBot) statsHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	_, args := bothelpers.ParseCommand(update.Message.Text)
	duration, ok := parsePeriod(args)
	if !ok {
		duration = 24 * time.Hour
	}

	// Chats data
	totalChats, err := ab.container.Chat.Count()
	if err != nil {
		ab.Report().Log().Err(err).Msg("Failed to get chat count")
		return
	}
	privateChatCount, err := ab.container.Chat.CountPrivate()
	groupChatCount := totalChats - privateChatCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get private chat count")
		privateChatCount = -1
		groupChatCount = -1
	}
	inactiveCount, err := ab.container.Chat.CountInactive(duration)
	activeCount := totalChats - inactiveCount
	if err != nil {
		log.Error().Err(err).Msg("Failed to get inactive chat count")
		inactiveCount = -1
		activeCount = -1
	}
	chatsNewCount, err := ab.container.Chat.CountNew(duration)
	if err != nil {
		ab.Report().Log().Err(err).Msg("Failed to get new chats count")
		return
	}

	chatsNewGrouped, err := ab.GetNewChatsGroupedByYear(duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get new chats grouped")
	}

	// Groups data
	groupCount, err := ab.container.Chat.CountUniqueConfiguredGroups()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get configured group count")
		groupCount = -1
	}
	chatsPerGroupAvg, err := ab.container.Chat.GetAvgChatsPerGroup()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get average chats per group")
		chatsPerGroupAvg = -1
	}
	chatsPerGroupMedian, err := ab.container.Chat.GetMedianChatsPerGroup()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get median chats per group")
		chatsPerGroupMedian = -1
	}

	// Updates data
	updateLogs, err := ab.container.Log.UpdateLogsByPeriod(time.Now().Add(-duration), time.Now())
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
	totalSendings, sendingOkCount, sendingFailCount, err := ab.container.Log.CountSendingLogs(model.SendingLogAny, duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get total sending logs")
	}
	dailySendings, _, _, err := ab.container.Log.CountSendingLogs(model.SendingLogDaily, duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get daily sending logs")
	}
	pairSendings, _, _, err := ab.container.Log.CountSendingLogs(model.SendingLogPair, duration)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get pair sending logs")
	}
	updateSendings, _, _, err := ab.container.Log.CountSendingLogs(model.SendingLogChange, duration)
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

		groupsTotal:         groupCount,
		chatsPerGroupAvg:    chatsPerGroupAvg,
		chatsPerGroupMedian: chatsPerGroupMedian,

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
	chatsNewGrouped map[int]int // Admission Year -> Count

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
	for year, count := range gr.chatsNewGrouped {
		fmt.Fprintf(&textNewChatsGrouped, "%d => %d\n", year, count)
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
	dailyTimes, err := ab.container.Chat.CountDailyTimeGrouped()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats grouped by daily sending time")
		return
	}

	dailyEnabledCount, err := ab.container.Chat.CountDailySendingOn()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with daily sending time enabled")
		return
	}

	pairEnabledCount, err := ab.container.Chat.CountPairNotificationOn()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with pair sending enabled")
		return
	}

	changeEnabledCount, err := ab.container.Chat.CountChangeAlertOn()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with update sending enabled")
		return
	}

	darkModeEnabledCount, err := ab.container.Chat.CountDarkModeOn()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats with dark mode enabled")
		return
	}

	timeKeys := make([]string, 0, len(dailyTimes))
	for k := range dailyTimes {
		timeKeys = append(timeKeys, k)
	}
	sort.Strings(timeKeys)

	var text strings.Builder
	fmt.Fprintf(&text, `CONFIGS

Dark mode enabled: %d
Pair enabled: %d
Daily enabled: %d
Update enabled: %d
Times:
<pre>`,
		darkModeEnabledCount, pairEnabledCount, dailyEnabledCount, changeEnabledCount)
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
	distribution, err := ab.container.Log.UpdateDist(dataKind, distPeriod, dur)
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

// GetNewChatsGroupedByYear returns a map of new chats grouped by the year of admission.
func (ab *AdminBot) GetNewChatsGroupedByYear(dur time.Duration) (map[int]int, error) {
	chats, err := ab.container.Chat.AllNew(dur)
	if err != nil {
		return nil, err
	}
	groupedChats := make(map[int]int)
	for _, chat := range chats {
		groupName := refutil.DerefOrTypeDefault(chat.GroupName)
		_, year, _, _, err := groupName.Parse()
		if err != nil {
			log.Error().Err(err).Msg("invalid group name")
			continue
		}

		groupedChats[year]++
	}
	return groupedChats, nil
}

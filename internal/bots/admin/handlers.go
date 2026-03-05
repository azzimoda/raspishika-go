package adminbot

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
	"github.com/azzimoda/raspishika-go/pkg/utils"
)

func (ab *AdminBot) registerHandlers() {
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommandStartOnly, ab.startHandler)

	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "chat", bot.MatchTypeCommand, ab.chatHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "group", bot.MatchTypeCommand, ab.groupHandler)

	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "stats", bot.MatchTypeCommand, ab.statsHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "config", bot.MatchTypeCommand, ab.configHandler)
	ab.bot.RegisterHandler(bot.HandlerTypeMessageText, "/dist_", bot.MatchTypePrefix, ab.distHandler)

	ab.bot.RegisterHandlerMatchFunc(func(update *tgmodels.Update) bool {
		_, err := models.ValidateGroupName(ab.services.Repo.DB, update.Message.Text)
		return err == nil
	}, ab.groupHandler)
	// TODO: Handle username and chat_id.
}

func (ab *AdminBot) defaultHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Debug().Msgf("Received update: %v", update)

	if update.Message != nil {
		if strings.HasPrefix(update.Message.Text, "/") {
			// Remove not handled command.
			bothelpers.DeleteMessageSafely(ctx, b, update.Message)
		}
	}
}

func (ab *AdminBot) startHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Welcome back, Master!",
	})
}

func (ab *AdminBot) chatHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	_, args := bothelpers.ParseCommand(update.Message.Text)

	tgChatID, err := strconv.ParseInt(args, 10, 64)
	var chat *models.Chat
	if err != nil {
		// Get last chat from database.
		chats, err := models.GetChats(ab.services.Repo.DB)
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
		chat, err = models.GetChatByTgChatID(ab.services.Repo.DB, tgChatID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get chat by chat ID")
			return
		}
	}

	ab.sendChatReport(chat, b, ctx, update)
}

const chatReportTemplateStr = `ID: {{.ID}}
Chat ID: <code>{{.TgChatID}}</code>
Username: @{{.UserName}}
State: {{.State}}
Department: <code>{{.DepartmentName}}</code>
Group: <code>{{.GroupName}}</code>
Daily Sending Time: <code>{{.DailySendingTime}}</code>
Pair Notificaton: {{.PairSending}}
ChangeAlert: {{.ChangeAlert}}
Access: {{.Access}}
Recent Teachers: {{.RecentTeachers}}
Recent updates:
<pre language="yaml">{{.RecentUpdates}}</pre>`

var chatReportTemplate, _ = template.New("report").Parse(chatReportTemplateStr)

func (ab *AdminBot) sendChatReport(chat *models.Chat, b *bot.Bot, ctx context.Context, update *tgmodels.Update) {
	log.Trace().Msg("sendChatReport")

	recentTeachers, err := models.GetTeacherByChatID(ab.services.Repo.DB, chat.ID)
	if err != nil {
		recentTeachers = []models.Teacher{}
		ab.services.Reporter.Report().Err(err).Msgf("Failed to get recent teachers for chat %d", chat.ID)
	}
	recentTeacherNames := make([]string, len(recentTeachers))
	for i, t := range recentTeachers {
		recentTeacherNames[i] = t.Name
	}
	recentTeachersStr := strings.Join(recentTeacherNames, ", ")

	recentUpdates, err := models.GetRecentChatUpdateLogs(ab.services.Repo.DB, chat.ID, 48*time.Hour)
	if err != nil {
		log.Error().Err(err).Int("chat.ID", chat.ID).Msg("Failed to get recent updates for chat")
		recentUpdates = []models.UpdateLog{}
	}
	var recentUpdatesStr strings.Builder
	for i := len(recentUpdates) - 1; i >= 0 && i >= len(recentUpdates)-5; i-- {
		fmt.Fprintf(&recentUpdatesStr, "- data: %v\n  time: %dms\n  error: %s\n",
			recentUpdates[i].Data,
			recentUpdates[i].HandlingTime,
			utils.DerefOrTypeDefault(recentUpdates[i].Error),
		)
	}

	log.Trace().Msg("Building report...")
	var buf bytes.Buffer
	chatReportTemplate.Execute(&buf, struct {
		*models.Chat
		RecentTeachers string
		RecentUpdates  string
	}{
		Chat:           chat,
		RecentTeachers: recentTeachersStr,
		RecentUpdates:  recentUpdatesStr.String(),
	})
	text := buf.String()

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
	if err != nil {
		ab.Report().Log().Err(err).Debug("text", text).Send()
	}
}

func (ab *AdminBot) groupHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	group := update.Message.Text // For default handler
	if strings.HasPrefix(update.Message.Text, "/") {
		_, group = bothelpers.ParseCommand(update.Message.Text) // For /group command
	}

	group, err := models.ValidateGroupName(ab.services.Repo.DB, update.Message.Text)
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate group name format")
		return
	}

	chats, err := models.GetChatsByGroup(ab.services.Repo.DB, group)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chats by group")
		return
	}

	if len(chats) == 0 {
		log.Error().Msgf("No chats found for group %s", group)
		return
	}

	var text strings.Builder
	fmt.Fprintf(&text, "Chats in group <code>%s</code> (%d chats):\n", group, len(chats))
	for _, chat := range chats {
		fmt.Fprintf(&text, "• <code>/chat %d</code>\n", chat.TgChatID)
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text.String(),
		ParseMode: tgmodels.ParseModeHTML,
	})
}

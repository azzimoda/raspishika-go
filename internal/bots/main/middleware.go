package mainbot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/azzimoda/raspishika-go/internal/models"
)

type contextKey string

const (
	chatContextKey      contextKey = "chat"
	errorContextKey     contextKey = "error"
	noLogFlagContextKey contextKey = "default_handler"
)

var (
	ErrUnknownUpdateType = fmt.Errorf("unknown update type")
)

// ensureChatMiddleware creates or updates chat in database before handling message.
//
// Use it as global middleware.
func (mb *MainBot) ensureChatMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		chat, err := mb.ensureChat(b, update)
		if errors.Is(err, ErrUnknownUpdateType) {
			log.Warn().Msg("Unknown update type") // TODO: Do I really need this log?
			next(ctx, b, update)
			return
		} else if err != nil {
			mb.services.Reporter.Report().Log().Err(err).Msg("Failed to create or update chat")
			return
		}

		ctx = context.WithValue(ctx, chatContextKey, chat)
		next(ctx, b, update)
	}
}

func (mb *MainBot) ensureChat(b *bot.Bot, update *tgmodels.Update) (*models.Chat, error) {
	var chatID models.ChatID
	var username models.UserName
	if update.Message != nil {
		chatID = models.ChatID(update.Message.Chat.ID)
		username = models.UserName(update.Message.Chat.Username)
	} else if update.CallbackQuery != nil {
		chatID = models.ChatID(update.CallbackQuery.Message.Message.Chat.ID)
		username = models.UserName(update.CallbackQuery.Message.Message.Chat.Username)
	} else {
		return nil, ErrUnknownUpdateType
	}

	chat, created, err := models.CreateOrUpdateChat(mb.services.Repository.DB, chatID, username)
	if err != nil {
		mb.services.Reporter.Report().Log().Err(err).Chat(models.Chat{TgChatID: chatID, UserName: &username}).
			Msg("Failed to create or update chat")
		return nil, fmt.Errorf("failed to create or update chat: %w", err)
	}
	if created {
		go mb.sendNewChatReport(chat, err, chatID, b)
	}

	return chat, nil
}

// sendNewChatReport sends a report to the admin chat when a new user chat is registered.
// It also sends a message to the admin chat if the user chat has a group configured.
func (mb *MainBot) sendNewChatReport(chat *models.Chat, err error, tgChatID models.ChatID, b *bot.Bot) {
	report, sentErr := mb.services.Reporter.Report().Chat(chat).Msg("New chat registered")
	if err != nil {
		log.Error().Err(err).Msg("Failed to send report")
	}

	msg := report.Message
	for range 5 {
		time.Sleep(20 * time.Second)

		if chat, err := models.GetChatByTgChatID(mb.services.Repository.DB, tgChatID); err == nil && chat.GroupName != nil {
			if sentErr == nil {
				b.DeleteMessage(context.Background(), &bot.DeleteMessageParams{ChatID: msg.Chat.ID, MessageID: msg.ID})
			}

			mb.services.Reporter.Report().Chat(chat).Msgf("Chat configured group %s", *chat.GroupName)
			break
		}
	}
}

// ignoreOldMessagesMiddleware ignores messages sent more that 10 minutes ago.
//
// Use it as global middleware.
func (mb *MainBot) ignoreOldMessagesMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		if update.Message == nil || time.Unix(int64(update.Message.Date), 0).After(time.Now().Add(-10*time.Minute)) {
			next(ctx, b, update)
			return
		}
		log.Trace().Msg("Old message ignored")
	}
}

// ignoreInaccessibleMessageCQMiddleware filters out callback queries with inaccessible messages.
func (mb *MainBot) ignoreInaccessibleMessageCQMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		if update.CallbackQuery == nil || update.CallbackQuery.Message.Message != nil {
			next(ctx, b, update)
			return
		}
	}
}

// callbackSF is a single flight group for handling callback queries
// and preventing them from being handled multiple times simultaneously for one message.
var callbackSF = singleflight.Group{}

// callbackQuerySingleFlightMiddleware ensures that a callback query is handled only once for one message.
//
// Use it as global middleware.
func (mb *MainBot) callbackQuerySingleFlightMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		if update.CallbackQuery == nil {
			next(ctx, b, update)
			return
		}

		key := fmt.Sprint(update.CallbackQuery.Message.Message.ID)
		_, err, shared := callbackSF.Do(key, func() (any, error) {
			next(ctx, b, update)
			return nil, nil
		})
		if err != nil {
			log.Error().Err(err).Str("message_id", key).Msg("Failed to handle a callback query in single flight")
		}
		if shared {
			log.Trace().Str("message_id", key).Msg("Prevented a callback query from being handled multiple times")
		}
	}
}

// logMiddleware logs incoming updates.
//
// Use it as last global middleware.
func (mb *MainBot) logMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, bot *bot.Bot, update *tgmodels.Update) {
		log.Trace().Any("update", update).Msg("Received update")

		var handlerErrs []error
		ctx = context.WithValue(ctx, errorContextKey, &handlerErrs)

		var noLogFlag bool
		ctx = context.WithValue(ctx, noLogFlagContextKey, &noLogFlag)

		startTime := time.Now()
		next(ctx, bot, update)
		elapsedTime := time.Since(startTime)

		log.Trace().Any("update", update).Msg("Update processed")

		if noLogFlag {
			return
		}

		chat, ok := ctx.Value(chatContextKey).(*models.Chat)
		if !ok {
			mb.services.Reporter.Report().Log().Msg("Failed to get chat from context")
			return
		}

		updateKind := "unknown"
		messageID := 0
		updateData := ""

		logEvent := log.Info().Dur("elapsed_time", elapsedTime)
		if update.Message != nil {
			message := update.Message
			updateKind = "message"
			updateData = message.Text
			logEvent.
				Int64("chat_id", message.Chat.ID).
				Str("username", message.From.Username).
				Str("first_name", message.From.FirstName).
				Str("last_name", message.From.LastName).
				Str("text", shortenText(message.Text, 100)).
				Msg("Message handled")
		} else if update.CallbackQuery != nil {
			updateKind = "callback_query"
			messageID = update.CallbackQuery.Message.Message.ID
			updateData = update.CallbackQuery.Data
			logEvent.
				Int("message_id", update.CallbackQuery.Message.Message.ID).
				Int64("chat_id", update.CallbackQuery.Message.Message.Chat.ID).
				Str("username", update.CallbackQuery.From.Username).
				Str("first_name", update.CallbackQuery.From.FirstName).
				Str("last_name", update.CallbackQuery.From.LastName).
				Str("data", update.CallbackQuery.Data).
				Msg("Callback query handled")
		} else {
			logEvent.Msg("Unknown update type")
		}

		var handlerErr error
		handlerErrStr := ""
		if len(handlerErrs) > 0 {
			log.Trace().Errs("errs", handlerErrs).Send()
			handlerErr = errors.Join(handlerErrs...)
			handlerErrStr = handlerErr.Error()
			mb.services.Reporter.Report().Log().Err(handlerErr).Chat(chat).
				Debug("update_type", updateKind).
				Debug("update_data", updateData).
				Msg("Handler error")
		}

		models.InsertUpdateLog(mb.services.Repository.DB, &models.UpdateLog{
			ChatID:       chat.ID,
			Kind:         updateKind,
			MessageID:    messageID,
			Data:         updateData,
			HandlingTime: int(elapsedTime.Milliseconds()),
			Error:        &handlerErrStr,
		})
	}
}

// checkConfigAccessMiddleware checks if user is allowed to use config commands in chat.
//
// Use it as middleware for config command and callback query handlers.
func (mb *MainBot) checkConfigAccessMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		chat, ok := ctx.Value(chatContextKey).(*models.Chat)
		if !ok {
			mb.services.Reporter.Report().Log().Msg("Failed to get chat from context")
			return
		}

		isAdmin, err := mb.isAdmin(ctx, b, update)
		if err != nil {
			mb.services.Reporter.Report().Log().Err(err).Chat(chat).Msg("Failed to get chat member")
			next(ctx, b, update)
			return
		}

		// User can use a config command only if chat access is ChatAccessAll, or the user is admin.
		logEvent := log.Trace().Bool("admin", isAdmin).Int("access", int(chat.Access))
		if chat.Access == models.ChatAccessAll || isAdmin {
			next(ctx, b, update)
			return
		} else {
			logEvent.Msg("User is not allowed to use config commands")
			if update.CallbackQuery != nil {
				mb.Bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
					CallbackQueryID: update.CallbackQuery.ID,
					Text:            "Доступ запрещен",
				})
			}
		}
	}
}

// checkRegularAccessMiddleware checks if user is allowed to use regular commands in chat.
//
// Use it as middleware for regular command handlers.
func (mb *MainBot) checkRegularAccessMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
		chat, ok := ctx.Value(chatContextKey).(*models.Chat)
		if !ok {
			mb.services.Reporter.Report().Log().Msg("Failed to get chat from context")
			return
		}

		isAdmin, err := mb.isAdmin(ctx, b, update)
		if err != nil {
			mb.services.Reporter.Report().Log().Err(err).Chat(chat).Msg("Failed to get chat member")
			next(ctx, b, update)
			return
		}

		// User can use a regular command only if chat access is not ChatAccessAdminOnly, or the user is admin.
		logEvent := log.Trace().Bool("admin", isAdmin).Int("access", int(chat.Access))
		if chat.Access != models.ChatAccessAdminOnly || isAdmin {
			next(ctx, b, update)
		}
		logEvent.Msg("User is not allowed to use regular commands")
	}
}

func (mb *MainBot) isAdmin(ctx context.Context, b *bot.Bot, update *tgmodels.Update) (bool, error) {
	var chatID, userID int64
	if update.Message != nil {
		chatID = update.Message.Chat.ID
		userID = update.Message.From.ID
	} else if update.CallbackQuery != nil {
		chatID = update.CallbackQuery.Message.Message.Chat.ID
		userID = update.CallbackQuery.From.ID
	}

	chatMember, err := b.GetChatMember(ctx, &bot.GetChatMemberParams{ChatID: chatID, UserID: userID})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get chat member; retrying...")
		// Retry once
		chatMember, err = b.GetChatMember(ctx, &bot.GetChatMemberParams{ChatID: chatID, UserID: userID})
		if err != nil {
			return false, fmt.Errorf("failed to get chat member with retry: %w", err)
		}
	}

	return chatMember.Type == tgmodels.ChatMemberTypeAdministrator || chatMember.Type == tgmodels.ChatMemberTypeOwner, nil
}

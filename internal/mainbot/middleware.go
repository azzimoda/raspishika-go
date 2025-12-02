package mainbot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"

	"github.com/azzimoda/raspishika-go/internal/database"
)

type contextKey string

const (
	chatContextKey           contextKey = "chat"
	errorContextKey          contextKey = "error"
	defaultHandlerContextKey contextKey = "default_handler"
)

var (
	ErrUnknownUpdateType = fmt.Errorf("unknown update type")
)

// ensureChatMiddleware creates or updates chat in database before handling message.
//
// Use it as global middleware.
func (mb *MainBot) ensureChatMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Trace().Msg("Middleware: ensureChatMiddleware")

		chat, err := mb.ensureChat(b, update)
		if errors.Is(err, ErrUnknownUpdateType) {
			log.Warn().Msg("Unknown update type")
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

func (mb *MainBot) ensureChat(b *bot.Bot, update *models.Update) (*database.Chat, error) {
	var chatID int64
	var username string
	if update.Message != nil {
		chatID = update.Message.Chat.ID
		username = update.Message.Chat.Username
	} else if update.CallbackQuery != nil {
		chatID = update.CallbackQuery.Message.Message.Chat.ID
		username = update.CallbackQuery.Message.Message.Chat.Username
	} else {
		return nil, ErrUnknownUpdateType
	}

	chat, created, err := mb.services.Repo.CreateOrUpdateChat(chatID, username)
	if err != nil {
		mb.services.Reporter.Report().Log().Err(err).Chat(database.Chat{TgChatID: chatID, UserName: &username}).
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
func (mb *MainBot) sendNewChatReport(chat *database.Chat, err error, tgChatID int64, b *bot.Bot) {
	msg, sentErr := mb.services.Reporter.Report().Chat(chat).Msg("New chat registered")
	if err != nil {
		log.Error().Err(err).Msg("Failed to send report")
	}

	time.Sleep(20 * time.Second)
	if chat, err := mb.services.Repo.GetChatByTgChatID(tgChatID); err == nil && chat.GroupName != nil {
		if sentErr == nil {
			b.DeleteMessage(context.Background(), &bot.DeleteMessageParams{ChatID: msg.Chat.ID, MessageID: msg.ID})
		}

		mb.services.Reporter.Report().Chat(chat).Msgf("Chat configured group %s", *chat.GroupName)
	}
}

// ignoreOldMessagesMiddleware ignores old messages.
//
// Use it as global middleware.
func (mb *MainBot) ignoreOldMessagesMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Trace().Msg("Middleware: ignoreOldMessagesMiddleware")

		if update.Message == nil || time.Unix(int64(update.Message.Date), 0).After(time.Now().Add(-10*time.Minute)) {
			next(ctx, b, update)
		}
	}
}

// TODO: Maybe I should not define callbackSF in global scope.

// callbackSF is a single flight group for handling callback queries
// and preventing them from being handled multiple times simultaneously for one message.
var callbackSF = singleflight.Group{}

// callbackQuerySingleFlightMiddleware ensures that a callback query is handled only once for one message.
//
// Use it as global middleware.
func (mb *MainBot) callbackQuerySingleFlightMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Trace().Msg("Middleware: callbackQuerySingleFlightMiddleware")

		if update.CallbackQuery == nil {
			next(ctx, b, update)
			return
		}

		key := fmt.Sprint(update.CallbackQuery.Message.Message.ID)
		_, err, shared := callbackSF.Do(key, func() (any, error) {
			log.Trace().Str("message_id", key).Msg("Handling a callback query")
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
	return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
		log.Trace().Any("update", update).Msg("Received update")

		var handlerErrs []error
		ctx = context.WithValue(ctx, errorContextKey, &handlerErrs)

		var defaultHandlerFlag bool
		ctx = context.WithValue(ctx, defaultHandlerContextKey, &defaultHandlerFlag)

		startTime := time.Now()
		next(ctx, bot, update)
		elapsedTime := time.Since(startTime)

		log.Trace().Any("update", update).Msg("Update processed")

		if defaultHandlerFlag {
			return
		}

		chat, ok := ctx.Value(chatContextKey).(*database.Chat)
		if !ok {
			mb.services.Reporter.Report().Log().Msg("Failed to get chat from context")
			return
		}

		updateKind := "unknown"
		messageID := 0
		updateData := ""

		logEvent := log.Info().Dur("elapsed_time", elapsedTime)
		if update.Message != nil {
			updateKind = "message"
			updateData = update.Message.Text
			logEvent.
				Int64("chat_id", update.Message.Chat.ID).
				Str("username", update.Message.From.Username).
				Str("first_name", update.Message.From.FirstName).
				Str("last_name", update.Message.From.LastName).
				Str("text", shortenText(update.Message.Text, 100)).
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

		// NOTE: I may want to implement skipping of some kinds of errors here.

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

		mb.services.Repo.InsertUpdateLog(&database.UpdateLog{
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
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Trace().Msg("Middleware: checkConfigAccessMiddleware")

		chat, ok := ctx.Value(chatContextKey).(*database.Chat)
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
		if chat.Access == database.ChatAccessAll || isAdmin {
			logEvent.Msg("User is allowed to use config commands")
			next(ctx, b, update)
		}
		logEvent.Msg("User is not allowed to use config commands")
	}
}

// checkRegularAccessMiddleware checks if user is allowed to use regular commands in chat.
//
// Use it as middleware for regular command handlers.
func (mb *MainBot) checkRegularAccessMiddleware(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		log.Trace().Msg("Middleware: checkRegularAccessMiddleware")

		chat, ok := ctx.Value(chatContextKey).(*database.Chat)
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
		if chat.Access != database.ChatAccessAdminOnly || isAdmin {
			logEvent.Msg("User is allowed to use regular commands")
			next(ctx, b, update)
		}
		logEvent.Msg("User is not allowed to use regular commands")
	}
}

func (mb *MainBot) isAdmin(ctx context.Context, b *bot.Bot, update *models.Update) (bool, error) {
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
		return false, fmt.Errorf("failed to get chat member: %w", err)
	}

	return chatMember.Type == models.ChatMemberTypeAdministrator || chatMember.Type == models.ChatMemberTypeOwner, nil
}

func shortenText(text string, maxLength int) string {
	if len(text) > maxLength {
		return text[:maxLength-2] + "…"
	}
	return text
}

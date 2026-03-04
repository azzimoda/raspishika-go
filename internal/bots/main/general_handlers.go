package mainbot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/pkg/bothelpers"
)

const StartMessageText = `Привет! Со мной ты можешь легко получить расписание своей группы и любого преподавателя

Для начала нужно задать свою группу, после этого можно будет использовать команды /week, /tomorrow, /left и настроить рассылки. Другие комманды и функции перечислены в /help

Помимо команд можно использовать кнопки клавиатуры, а также меня можно добавить в групповой чат

Подпишись на канал разработчика @mazzaLLM, где ты можешь найти новости о боте и обсудить бота в комментариях`

const HelpMessageText = `Основные команды:

• /week — Расписание на неделю
• /tomorrow — Расписание на завтра
• /left — Оставшиеся сегодня пары
• /teacher — Расписание преподавателя
• /settings — Меню настроек
• /stop — Удалить данные о себе и остановить рассылки
• /help — Это сообщение

Доступные настройки:

• Ежедневная рассылка: можно задать время, в которое бот будет присылать расписание на неделю каждый день: /time, /daily_off
• Напоминания за 15 минут перед парами: /reminder_on, /reminder_off
• Уведомления об изменениях в расписании: /alert_on, /alert_off
• В групповом чате можно настроить уровен доступа участников к командам: /access

Также:

• Бота можно добавить в групповой чат
• Можно получить расписание любой группы, просто прислав её название, например: "ИСПт-22-(9)-2" или "испт 22 9 2"

По всем вопросам пишите в комментарии или директ канала @mazzaLLM.`

// TODO: Move this out of here into separate handler. It should not be here.
func (mb *MainBot) defaultHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Default handler")

	if update.Message != nil {
		log.Trace().Str("message", update.Message.Text).Msg("Unhandled message")
		if groupName, err := models.ValidateGroupName(mb.services.Repo.DB, update.Message.Text); err == nil {
			mb.sendQuickGroupSchedule(ctx, groupName, update, b)
		} // Else just ignore message
	}

	if update.CallbackQuery != nil {
		_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Это сообщение больше не поддерживается",
		})
		addContextHandlerError(ctx, err)
	}

	notLogFlag := ctx.Value(noLogFlagContextKey).(*bool)
	*notLogFlag = true
}

// TODO: Move this handler into separate file.
func (mb *MainBot) sendQuickGroupSchedule(ctx context.Context, groupName string, update *tgmodels.Update, b *bot.Bot) {
	log.Trace().Str("groupName", groupName).Msg("Sending quick group week schedule")

	chat, err := models.GetChatByTgChatID(mb.services.Repo.DB, update.Message.Chat.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chat from DB")
		return
	}

	group, err := models.GetGroupByName(mb.services.Repo.DB, groupName)
	if err != nil {
		log.Error().Str("groupName", groupName).Err(err).
			Msg("Failed get group from DB for quick group week schedule")
		return
	}

	_, err = b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Action:          tgmodels.ChatActionTyping,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send chat action")
	}

	conf := models.GroupScheduleConfig(group)
	imageFilename, imageData, err := mb.PrepareScheduleImage(conf)
	if err != nil {
		log.Error().Any("group", group).Err(err).
			Msg("Failed to prepare week schedule data for quick group schedule")
		return
	}

	err = mb.SendWeekScheduleMessages(
		ctx,
		b,
		update.Message.MessageThreadID,
		chat,
		conf,
		imageFilename,
		imageData,
	)
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) startHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Start handler")
	_, err := mb.Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            StartMessageText,
	})
	addContextHandlerError(ctx, err)

	if err == nil {
		chat, err := models.GetChatByTgChatID(mb.services.Repo.DB, update.Message.Chat.ID)
		if err != nil {
			// Error: failed to get chat by chat ID: %!w(<nil>)
			errFormatted := fmt.Errorf("failed to get chat by chat ID: %w", err)
			log.Warn().Err(err).Err(errFormatted).Msg("Failed to get chat by chat ID")
			addContextHandlerError(ctx, errFormatted)
		} else if chat.GroupName == nil {
			mb.groupHandler(ctx, b, update)
		}
	}
}

func (mb *MainBot) helpHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Help handler")
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            HelpMessageText,
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) stopHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Stop handler")

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		addContextHandlerError(ctx, ErrNoChatContext)
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Не удалось удалить данные чата, попробуйте позже",
		})
		return
	}

	if err := models.DeleteChat(mb.services.Repo.DB, chat.ID); err != nil {
		addContextHandlerError(ctx, fmt.Errorf("failed to delete chat: %w", err))
		sendErrorMessage(ctx, b, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text:            "Не удалось удалить данные чата, попробуйте позже",
		})
		return
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Рассылки остановлены и ваши данные удалены. Спасибо, что пользовались нашим ботом!",
		ReplyMarkup:     tgmodels.ReplyKeyboardRemove{RemoveKeyboard: true},
	})
	addContextHandlerError(ctx, err)
}

func (mb *MainBot) deleteHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Delete handler")

	_, err := bothelpers.DeleteMessageSafely(ctx, b, update.CallbackQuery.Message.Message)
	addContextHandlerError(ctx, err)

	if repoErr := models.UpdateChatState(mb.services.Repo.DB,
		update.CallbackQuery.Message.Message.Chat.ID,
		models.ChatStateDefault,
	); repoErr != nil {
		mb.services.Reporter.Report().Log().Err(repoErr).Chat(update.CallbackQuery.Message.Message.Chat.ID).
			Msg("Error in deleteHandler")
		addContextHandlerError(ctx, fmt.Errorf("failed to update chat state: %w", repoErr))
		return
	}
}

func (mb *MainBot) textCancelHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	log.Trace().Msg("Text cancel handler")

	_, err := bothelpers.DeleteMessageSafely(ctx, b, update.Message)
	addContextHandlerError(ctx, err)

	chat, ok := ctx.Value(chatContextKey).(*models.Chat)
	if !ok {
		mb.services.Reporter.Report().Log().Err(fmt.Errorf("failed to get chat from context")).
			Chat(update.Message.Chat.ID).Msg("Error in textCancelHandler")
		return
	}

	chat.State = models.ChatStateDefault
	if err := chat.Update(mb.services.Repo.DB); err != nil {
		mb.services.Reporter.Report().Log().Err(err).Chat(update.Message.Chat.ID).Msg("Error in textCancelHandler")
		addContextHandlerError(ctx, fmt.Errorf("failed to update chat: %w", err))
		return
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		Text:            "Действие отменено",
		ReplyMarkup:     mainMenuReplyMarkup(update.Message.Chat.ID > 0),
	})
	addContextHandlerError(ctx, err)
}

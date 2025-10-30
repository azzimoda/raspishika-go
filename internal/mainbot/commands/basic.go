package commands

import (
	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

// TODO: Rewrite start message.
const StartMessage = `Привет! Я предостовляю удобный способ получения расписания МПК ТИУ.

Для начала нужно задать свою группу для использования комманд /week, /tomorrow, /left и получения рассылки. Други комманды и функции перечислены в /help.

Помимо команд можно использовать кнопки клавиатуры, а также меня можно добавить в групповой чат.

Подпишись на канал разработчика @mazzaLLM, где ты можешь найти новости о боте и поделиться своим мнением в комментариях.`

const HelpMessage = `Доступные команды:

• /week — Расписание на неделю
• /tomorrow — Расписание на завтра
• /left — Оставшиеся пары
• /quick — Расписание другой группы
• /teacher — Расписание преподавателя

• /settings — Меню настроек
• /group — Изменить свою группу
• /daily_time — Настроить ежедневную рассылку
• /daily_off — Выключить ежедневную рассылку
• /reminder_on — Включить напоминания перед парами
• /reminder_off — Выключить напоминания перед парами
• /access — Изменить уровень доступа к командам бота в групповом чате

• /stop — Удалить данные о себе и остановить рассылки
• /help — Это сообщение

Прочие функции:

• Бота можно добавить в групповой чат
• Напоминание приходит в течение 15 минут до начала пары

По всем вопросам обращайтесь к расработчику @MazzzaRellla или пишите в комментарии канала @mazzaLLM.`

func OnStart(
	api *tgbotapi.BotAPI,
	repo *database.Repository,
	browser *browser.BrowserService,
	cache *cache.Cache,
	msg *tgbotapi.Message,
) error {
	_, err := api.Send(tgbotapi.NewMessage(msg.Chat.ID, StartMessage))

	if err == nil {
		if chat, err := repo.GetChatByChatID(msg.Chat.ID); err == nil && chat.GroupName == nil {
			return OnGroup(api, repo, browser, cache, msg)
		} else {
			return err
		}
	}

	return err
}

func OnHelp(api *tgbotapi.BotAPI, msg *tgbotapi.Message) error {
	_, err := api.Send(tgbotapi.NewMessage(msg.Chat.ID, HelpMessage))
	return err
}

func OnStop(api *tgbotapi.BotAPI, repo *database.Repository, msg *tgbotapi.Message) error {
	chat, err := repo.GetChatByChatID(msg.Chat.ID)
	if err == nil {
		if err := repo.DeleteChat(chat.ID); err != nil {
			log.Error().Err(err).Int64("chatID", msg.Chat.ID).Msg("Failed to delete chat from DB")
		}
	} else {
		log.Error().Err(err).Int64("chatID", msg.Chat.ID).Msg("Failed to get chat by ID")
	}

	api.Send(tgbotapi.NewMessage(msg.Chat.ID, "Ваши данные удалены и рассылки выключены"))
	return err
}

package commands

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
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

func (ch *CommandHandler) OnStart(msg *tgbotapi.Message) error {
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, StartMessage)
	newMsg.ReplyMarkup = utils.MainMenuReplyMarkup(msg.Chat.IsPrivate())
	_, err := ch.Bot.API().Send(newMsg)

	if err == nil {
		if chat, repoErr := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID); repoErr == nil && chat.GroupName == nil {
			return ch.OnGroup(msg)
		} else {
			log.Warn().Err(repoErr).Int64("tgChatID", msg.Chat.ID).Msg("Failed to get chat by ID on /start")
			return err
		}
	}

	return err
}

func (ch *CommandHandler) OnHelp(msg *tgbotapi.Message) error {
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, HelpMessage)
	newMsg.ReplyMarkup = utils.MainMenuReplyMarkup(msg.Chat.IsPrivate())
	_, err := ch.Bot.API().Send(newMsg)
	return err
}

func (ch *CommandHandler) OnStop(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err == nil {
		if err := ch.Bot.Repo().DeleteChat(chat.ID); err != nil {
			log.Error().Err(err).Int64("tgChatID", msg.Chat.ID).Msg("Failed to delete chat from DB")
		}
	} else {
		log.Error().Err(err).Int64("tgChatID", msg.Chat.ID).Msg("Failed to get chat by ID")
	}

	newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Ваши данные удалены и рассылки выключены")
	newMsg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(false)
	ch.Bot.API().Send(newMsg)
	return err
}

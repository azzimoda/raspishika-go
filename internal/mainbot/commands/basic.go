package commands

import (
	"github.com/azzimoda/raspishika-go/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

// TODO: Rewrite start message.
const StartMessage = `Привет! Я предоставляю удобный способ получать расписание пар МПК ТИУ

Для этого тебе нужно задать группу с помощью команды /set_group. После этого ты можешь получать расписание на неделю (/week), на завтра (/tomorrow) или оставшиеся пары сегодня (/left).

Также ты можешь использовать кнопки клавиатуры и добавлять меня в группы. Остальные команды перечислены в /help.

По всем вопросам обращайтесь к расработчику @MazzzaRellla или пишите в комментарии канала @mazzaLLM.`

const HelpMessage = `Доступные команды:

- /left — Оставшиеся пары
- /tomorrow — Расписание на завтра
- /week — Расписание на неделю
- /quick — Расписание другой группы
- /teacher — Расписание преподавателя
- /settings — Меню настроек
- /group — Изменить свою группу
- /daily_time — Настроить ежедневную рассылку
- /daily_off — Выключить ежедневную рассылку
- /reminder_on — Включить уведомления перед парами
- /reminder_off — Выключить уведомления перед парами
- /access — Изменить уровень доступа к командам бота в группе
- /stop — Удалить данные о себе и остановить рассылки
- /help — Это сообщение

По всем вопросам обращайтесь к расработчику @MazzzaRellla или пишите в комментарии канала @mazzaLLM.`

func OnStart(api *tgbotapi.BotAPI, msg *tgbotapi.Message) error {
	_, err := api.Send(tgbotapi.NewMessage(msg.Chat.ID, StartMessage))
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

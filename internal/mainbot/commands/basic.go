package commands

import (
	"github.com/azzimoda/raspishika-go/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

const StartMessage = `Привет! Я предоставляю удобный способ получать расписание пар МПК ТИУ

Для этого тебе нужно задать группу с помощью команды /set_group. \
После этого ты можешь получать расписание на неделю (/week), на завтра (/tomorrow) или \
оставшиеся пары сегодня (/left).

Также ты можешь использовать кнопки клавиатуры и добавлять меня в группы. \
Остальные команды перечислены в /help.

По всем вопросам обращайтесь к расработчику @MazzzaRellla или пишите в комментарии канала @mazzaLLM.`

// TODO: Rewrite help message.
const HelpMessage = `Доступные команды:

- /left — Оставшиеся пары
- /tomorrow — Расписание на завтра
- /week — Расписание на неделю
- /quick — Расписание другой группы
- /teacher — Расписание преподавателя
- /daily_sending — Настроить ежедневную рассылку
- /daily_sending_off — Выключить ежедневную рассылку
- /pair_sending_on — Включить уведомления перед парами
- /pair_sending_off — Выключить уведомления перед парами
- /set_group — Изменить свою группу
- /access — Изменить уровень доступа к командам бота в группе
- /cancel — Отменить действие или выйти из меню
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

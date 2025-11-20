package commands

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	botutils "github.com/azzimoda/raspishika-go/internal/mainbot/utils"
)

func (ch *CommandHandler) OnReminder(msg *tgbotapi.Message, isOn bool) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d) %w", msg.Chat.ID, err)
	}

	chat.PairSending = isOn
	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat data: %w", err)
	}

	text := "Напоминания выключены"
	if isOn {
		text = "Напоминания включены"
	}
	_, err = ch.Bot.API().Send(tgbotapi.NewMessage(msg.Chat.ID, text))
	return err
}

func (ch *CommandHandler) OnAccess(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", msg.Chat.ID, err)
	}

	newMsg := tgbotapi.NewMessage(
		msg.Chat.ID,
		fmt.Sprintf(`Текущий уровень доступа: %d
		0 — без ограничений
		1 — настройки только для админов
		2 — все команды только для админов`, chat.Access),
	)
	newMsg.ReplyMarkup = botutils.AccessMenuInlineMarkup(chat.Access)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

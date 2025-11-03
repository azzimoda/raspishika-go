package callbacks

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (ch *CallbackHandler) OnConfigGroup(
	commandHandler *commands.CommandHandler,
	query *tgbotapi.CallbackQuery,
	args []string,
) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
	return commandHandler.OnGroup(query.Message) // Выглядит как костыль, но работает
}

func (ch *CallbackHandler) OnSelectDepartment(query *tgbotapi.CallbackQuery, args []string) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	groups, err := scraper.FetchDepartmentGroups(ch.Bot.Repo(), ch.Bot.Browser(), ch.Bot.Cache(), args[0])
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch groups: %w", err)
	}

	if err := ch.Bot.Repo().UpdateChatState(query.Message.Chat.ID, database.ChatStateSelectingGroup); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(query.Message.Chat.ID, "Выберите группу на клавиатуре")
	newMsg.ReplyMarkup = groupsReplyMarkup(groups)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func groupsReplyMarkup(groups []database.Group) tgbotapi.ReplyKeyboardMarkup {
	rows := make([][]tgbotapi.KeyboardButton, 0)

	rows = append(rows, tgbotapi.NewKeyboardButtonRow(tgbotapi.NewKeyboardButton("Отмена")))
	for i := 0; i < len(groups); i += 2 {
		row := make([]tgbotapi.KeyboardButton, 0)
		for j := i; j < len(groups) && j < i+2; j++ {
			row = append(row, tgbotapi.KeyboardButton{Text: groups[j].GroupName})
		}
		rows = append(rows, row)
	}

	return tgbotapi.ReplyKeyboardMarkup{
		Keyboard:        rows,
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
		// Selective: true,
	}
}

func (ch *CallbackHandler) OnConfigDailyTime(
	commandHandler *commands.CommandHandler,
	query *tgbotapi.CallbackQuery,
	args []string,
) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
	return commandHandler.OnDailyTime(query.Message) // Выглядит как костыль, но работает
}

func (ch *CallbackHandler) OnDailyOff(query *tgbotapi.CallbackQuery, args []string) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	chat, err := ch.Bot.Repo().GetChatByTgChatID(query.Message.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", query.Message.Chat.ID, err)
	}

	chat.DailySendingTime = ""
	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat (%d): %w", query.Message.Chat.ID, err)
	}

	return commands.SendSettingsMenu(ch.Bot.API(), chat, query.Message.Chat.ID) // TODO: Implement editing.
}

func (ch *CallbackHandler) OnConfigReminder(
	commandHandler *commands.CommandHandler,
	query *tgbotapi.CallbackQuery,
	args []string,
) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	chat, err := ch.Bot.Repo().GetChatByTgChatID(query.Message.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", query.Message.Chat.ID, err)
	}

	chat.PairSending = args[0] == "true"
	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
	}

	return commands.SendSettingsMenu(ch.Bot.API(), chat, query.Message.Chat.ID) // TODO: Implement editing.
}

func (ch *CallbackHandler) OnConfigAccess(
	commandHandler *commands.CommandHandler,
	query *tgbotapi.CallbackQuery,
	args []string,
) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	chat, err := ch.Bot.Repo().GetChatByTgChatID(query.Message.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", query.Message.Chat.ID, err)
	}

	chat.Access, err = strconv.Atoi(args[0])
	if err != nil {
		chat.Access = 0
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		log.Error().Err(err).Msg("failed to parse access level; fallback to 0")
	}

	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat (%d): %w", query.Message.Chat.ID, err)
	}

	return commands.SendSettingsMenu(ch.Bot.API(), chat, query.Message.Chat.ID) // TODO: Implement editing.
}

func (ch *CallbackHandler) OnSetAccess(query *tgbotapi.CallbackQuery, args []string) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(query.Message.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", query.Message.Chat.ID, err)
	}

	chat.Access, err = strconv.Atoi(args[0])
	if err != nil {
		chat.Access = 0
		log.Error().Err(err).Msg("failed to parse access level; fallback to 0")
	}
	if err := ch.Bot.Repo().UpdateChat(chat); err != nil {
		return fmt.Errorf("failed to update chat (%d): %w", query.Message.Chat.ID, err)
	}

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		query.Message.Chat.ID,
		query.Message.MessageID,
		fmt.Sprintf(`Текущий уровень доступа: %d
		0 — без ограничений
		1 — настройки только для админов
		2 — все команды только для админов`, chat.Access),
		utils.AccessMenuInlineMarkup(chat.Access),
	)
	_, err = ch.Bot.API().Send(editMsg)

	if err != nil && strings.Contains(err.Error(), "message is not modified") {
		ch.Bot.API().Send(tgbotapi.NewCallback(query.ID, "Ничего не изменилось"))
		log.Warn().Int64("tgChatID", query.Message.Chat.ID).Msg("message is not modified")
		return nil
	}
	return err
}

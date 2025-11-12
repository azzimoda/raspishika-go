package callbacks

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/internal/mainbot/commands"
	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"
)

func (ch *CallbackHandler) OnQuickSelectDepartment(query *tgbotapi.CallbackQuery, args []string) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	groups, err := scraper.FetchDepartmentGroups(ch.Bot.Repo(), ch.Bot.Browser(), ch.Bot.Cache(), args[0])
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to fetch groups: %w", err)
	}

	if err := ch.Bot.Repo().UpdateChatState(query.Message.Chat.ID, database.ChatStateQuickSelectingGroup); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	newMsg := tgbotapi.NewMessage(query.Message.Chat.ID, "Выберите группу на клавиатуре")
	newMsg.ReplyMarkup = groupsReplyMarkup(groups)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

func (ch *CallbackHandler) OnSelectTeacher(commandHandler *commands.CommandHandler, query *tgbotapi.CallbackQuery, args []string) error {
	ch.Bot.API().Send(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))

	chat, err := ch.Bot.Repo().GetChatByTgChatID(query.Message.Chat.ID)
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get chat by chat id (%d): %w", query.Message.Chat.ID, err)
	}

	teacher, err := ch.Bot.Repo().GetTeacherByTeacherID(args[0])
	if err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to get teacher by name (%s): %w", args[0], err)
	}

	if err := ch.Bot.Repo().AddChatRecentTeacher(chat.ID, teacher.ID); err != nil {
		log.Error().Err(err).Any("chat", chat).Any("teacher", teacher).Msg("Failed to add recent teacher")
		ch.Bot.Report().Chat(chat).Err(err).Sendf("Failed to add recent teacher %s", teacher.Name)
	}

	if err := ch.Bot.Repo().UpdateChatState(query.Message.Chat.ID, database.ChatStateDefault); err != nil {
		utils.SendErrorMessage(ch.Bot.API(), query.Message.Chat.ID, utils.ErrMsgTryLater)
		return fmt.Errorf("failed to update chat state: %w", err)
	}

	scheduleCfg := scraper.TeacherScheduleConfig(teacher)
	return commandHandler.SendWeekSchedule(chat, scheduleCfg)
}

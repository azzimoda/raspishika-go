package commands

import (
	"time"

	botutils "github.com/azzimoda/raspishika-go/internal/mainbot/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
)

func (ch *CommandHandler) OnLeft(msg *tgbotapi.Message) error {
	chat, err := ch.Bot.Repo().GetChatByTgChatID(msg.Chat.ID)
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgTryLater)
	}

	if chat.GroupName == nil {
		log.Warn().Msg("Group not set, offering to set group")
		// return ch.OnGroup(msg)
		return nil
	}

	if time.Now().Weekday() == time.Sunday {
		newMsg := tgbotapi.NewMessage(msg.Chat.ID, "Сегодня воскресенье, отдыхайте!")
		newMsg.ReplyMarkup = botutils.InlineButtonMarkupUpdate("left", *chat.GroupName)
		_, err := ch.Bot.API().Send(newMsg)
		return err
	}

	// group, shouldReturn, err := ch.tryGetGroup(chat, msg)
	// if shouldReturn {
	// 	return err
	// }

	// rawSchedule, err := ch.Bot.ScheduleManager().Get(
	// 	ch.Bot.Repo(),
	// 	ch.Bot.Browser(),
	// 	ch.Bot.Cache(),
	// 	scraper.GroupScheduleConfig(group),
	// )
	if err != nil {
		botutils.SendErrorMessage(ch.Bot.API(), msg.Chat.ID, botutils.ErrMsgFailedFetchSchedule)
		// return fmt.Errorf("failed to fetch schedule of group %s: %w", group.GroupName, err)
		return nil
	}

	// schedule := rawSchedule.Transform()
	// left := schedule.Days[0].Left()
	text := ""
	// if left.IsEmpty() {
	// 	text = "Сегодня больше нет пар"
	// } else {
	// 	text = left.String()
	// }
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, text)
	newMsg.ParseMode = tgbotapi.ModeMarkdownV2
	newMsg.ReplyMarkup = botutils.InlineButtonMarkupUpdate("left", *chat.GroupName)
	_, err = ch.Bot.API().Send(newMsg)
	return err
}

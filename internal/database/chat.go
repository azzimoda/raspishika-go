package database

import (
	"database/sql"
	"time"

	"github.com/rs/zerolog/log"
)

type ChatState string

const (
	ChatStateDefault                  ChatState = "default"
	ChatStateSelectingDepartment      ChatState = "selecting_department"
	ChatStateSelectingGroup           ChatState = "selecting_group"
	ChatStateQuickSelectingDepartment ChatState = "quick_selecting_department"
	ChatStateQuickSelectingGroup      ChatState = "quick_selecting_group"
	ChatStateSelectingTeacher         ChatState = "selecting_teacher"
	ChatStateSelectingTime            ChatState = "selecting_time"
)

type Chat struct {
	ID               int       `db:"id" json:"id"`
	ChatID           int64     `db:"chat_id" json:"chat_id"`
	UserName         *string   `db:"username" json:"username"`
	State            ChatState `db:"state" json:"state"`
	DepartmentName   *string   `db:"department" json:"department"`
	GroupName        *string   `db:"group" json:"group"`
	DailySendingTime string    `db:"daily_sending_time" json:"daily_sending_time"`
	PairSending      bool      `db:"pair_sending" json:"pair_sending"`
	Access           int       `db:"access" json:"access"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

func (c *Chat) IsPrivate() bool {
	return c.ChatID > 0
}

func (r *Repository) CreateChat(chatID int64, username string) (int64, error) {
	res, err := r.db.Exec(
		`INSERT INTO chats (chat_id, username)
		VALUES (?,?)`,
		chatID, username)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repository) CreateOrUpdateChat(chatID int64, username string) (*Chat, error) {
	var chat Chat
	err := r.db.Get(&chat, `SELECT * FROM chats WHERE chat_id = ?`, chatID)

	if err == sql.ErrNoRows {
		log.Debug().Int64("chatID", chatID).Msg("Chat does not exist, creating new one...")
		id, err := r.CreateChat(chatID, username)
		if err != nil {
			return nil, err
		}
		return r.GetChat(id)
	}

	if err != nil {
		log.Error().Err(err).Int64("chatID", chatID).Msg("Failed to get chat")
		return nil, err
	}

	log.Trace().Int64("chatID", chatID).Msg("Chat already exists")
	return &chat, nil
}

func (r *Repository) UpdateChat(chat *Chat) error {
	_, err := r.db.NamedExec(
		`UPDATE chats
		SET username = :username, state = :state, department = :department, "group" = :group,
		daily_sending_time = :daily_sending_time, pair_sending = :pair_sending, access = :access
		WHERE id = :id`,
		chat,
	)
	return err
}

func (r *Repository) UpdateChatState(chatID int64, state ChatState) error {
	_, err := r.db.Exec(`UPDATE chats SET state = ? WHERE chat_id = ?`, state, chatID)
	return err
}

func (r *Repository) GetChat(id int64) (*Chat, error) {
	var chat Chat
	if err := r.db.Get(&chat, `SELECT * FROM chats WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return &chat, nil
}

func (r *Repository) GetChatByChatID(chatID int64) (*Chat, error) {
	var chat Chat
	if err := r.db.Get(&chat, `SELECT * FROM chats WHERE chat_id = ?`, chatID); err != nil {
		return nil, err
	}
	return &chat, nil
}

func (r *Repository) GetChatsByDailySendingTime(timeStr string) ([]Chat, error) {
	var chats []Chat
	if err := r.db.Select(
		&chats, `SELECT * FROM chats WHERE "group" IS NOT NULL AND daily_sending_time = ?`, timeStr,
	); err != nil {
		return nil, err
	}
	return chats, nil
}

func (r *Repository) GetChatsWithPairSendingEnabled() (chats []Chat, err error) {
	err = r.db.Select(&chats, `SELECT * FROM chats WHERE "group" IS NOT NULL AND pair_sending = 1`)
	return
}

func (r *Repository) DeleteChat(id int) error {
	_, err := r.db.Exec(`DELETE FROM chats WHERE id = ?`, id)
	if err == nil {
		log.Debug().Int("Chat.ID", id).Msg("Chat is deleted")
	} else {
		log.Error().Err(err).Int("Chat.ID", id).Msg("Failed to delete chat")
	}
	return err
}

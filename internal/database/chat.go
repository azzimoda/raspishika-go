package database

import (
	"database/sql"
	"time"

	"github.com/rs/zerolog/log"
)

type Chat struct {
	ID               int64     `db:"id" json:"id"`
	ChatID           int64     `db:"chat_id" json:"chat_id"`
	UserName         *string   `db:"username" json:"username"`
	DepartmentName   *string   `db:"department" json:"department"`
	GroupName        *string   `db:"group" json:"group"`
	DailySendingTime string    `db:"daily_sending_time" json:"daily_sending_time"`
	PairSending      bool      `db:"pair_sending" json:"pair_sending"`
	Access           int       `db:"access" json:"access"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
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

func (r *Repository) DeleteChat(id int64) error {
	_, err := r.db.Exec(`DELETE FROM chats WHERE id = ?`, id)
	return err
}

package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/azzimoda/raspishika-go/pkg/utils"
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
	TgChatID         int64     `db:"tg_chat_id" json:"tg_chat_id"`
	UserName         *string   `db:"username" json:"username"`
	State            ChatState `db:"state" json:"state"`
	DepartmentName   *string   `db:"department" json:"department"`
	GroupName        *string   `db:"group" json:"group"`
	DailySendingTime *string   `db:"daily_sending_time" json:"daily_sending_time"`
	PairSending      bool      `db:"pair_sending" json:"pair_sending"`
	Access           int       `db:"access" json:"access"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

func (c *Chat) IsPrivate() bool {
	return c.TgChatID > 0
}

func (r *Repository) CreateChat(tgChatID int64, username string) (int64, error) {
	res, err := r.db.Exec(
		`INSERT INTO chats (tg_chat_id, username)
		VALUES (?,?)`,
		tgChatID, username)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreateOrUpdateChat creates or updates chat in the database.
// If chat does not exist, it creates a new one and returns true as second return value.
// If chat exists, it updates its username and returns false as second return value.
func (r *Repository) CreateOrUpdateChat(tgChatID int64, username string) (*Chat, bool, error) {
	var chat Chat
	err := r.db.Get(&chat, `SELECT * FROM chats WHERE tg_chat_id = ?`, tgChatID)

	if err == sql.ErrNoRows {
		// Create new chat.
		log.Debug().Int64("tgChatID", tgChatID).Msg("Chat does not exist, creating new one...")
		id, err := r.CreateChat(tgChatID, username)
		if err != nil {
			return nil, true, fmt.Errorf("failed to create chat (%d, %s): %w", tgChatID, username, err)
		}

		chat, err := r.GetChat(id)
		return chat, true, err
	}

	if err != nil {
		log.Error().Err(err).Int64("tgChatID", tgChatID).Msg("Failed to get chat")
		return nil, false, fmt.Errorf("failed to get chat by Telegram chat ID (%d): %w", tgChatID, err)
	}

	if utils.DerefOrTypeDefault(chat.UserName) != username {
		// Update username.
		chat.UserName = &username
		if err := r.UpdateChat(&chat); err != nil {
			err := fmt.Errorf("failed to update chat's username (%v -> %s): %w", chat.UserName, username, err)
			return nil, false, err
		}

		chat, err := r.GetChat(int64(chat.ID))
		return chat, false, err
	}

	// Return existing chat.
	log.Trace().Int64("tgChatID", tgChatID).Msg("Chat already exists")
	return &chat, false, nil
}

func (r *Repository) UpdateChat(chat *Chat) error {
	chat.UpdatedAt = time.Now()
	_, err := r.db.NamedExec(
		`UPDATE chats
		SET username = :username,
			state = :state,
			department = :department,
			"group" = :group,
			daily_sending_time = :daily_sending_time,
			pair_sending = :pair_sending,
			access = :access,
			updated_at = :updated_at
		WHERE id = :id`,
		chat,
	)
	return err
}

func (r *Repository) UpdateChatState(tgChatID int64, state ChatState) error {
	_, err := r.db.Exec(`UPDATE chats SET state = ?, updated_at = ? WHERE tg_chat_id = ?`, state, time.Now(), tgChatID)
	return err
}

func (r *Repository) GetChat(id int64) (*Chat, error) {
	var chat Chat
	if err := r.db.Get(&chat, `SELECT * FROM chats WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return &chat, nil
}

func (r *Repository) GetChatByTgChatID(tgChatID int64) (*Chat, error) {
	var chat Chat
	if err := r.db.Get(&chat, `SELECT * FROM chats WHERE tg_chat_id = ?`, tgChatID); err != nil {
		return nil, err
	}
	return &chat, nil
}

func (r *Repository) GetChatByUserName(username string) (*Chat, error) {
	var chat Chat
	if err := r.db.Get(&chat, `SELECT * FROM chats WHERE LOWER(username) = ?`, strings.ToLower(username)); err != nil {
		return nil, err
	}
	return &chat, nil
}

func (r *Repository) GetChats() ([]Chat, error) {
	var chats []Chat
	if err := r.db.Select(&chats, `SELECT * FROM chats`); err != nil {
		return nil, err
	}
	return chats, nil
}

func (r *Repository) GetChatsByGroup(group string) ([]Chat, error) {
	var chats []Chat
	if err := r.db.Select(&chats, `SELECT * FROM chats WHERE "group" = ?`, group); err != nil {
		return nil, err
	}
	return chats, nil
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
		log.Debug().Int("Chat.ID", id).Msg("Chat deleted")
	} else {
		log.Error().Err(err).Int("Chat.ID", id).Msg("Failed to delete chat")
	}
	return err
}

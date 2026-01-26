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
	ChatStateDefault             ChatState = "default"
	ChatStateSelectingDepartment ChatState = "selecting_department"
	ChatStateSelectingGroup      ChatState = "selecting_group"
	ChatStateSelectingTeacher    ChatState = "selecting_teacher"
	ChatStateSelectingTime       ChatState = "selecting_time"
)

type ChatAccessLevel int

// ChatAccess constants define the access level of a group chat.
const (
	// All commands are available for all users.
	ChatAccessAll ChatAccessLevel = 0
	// Configuration commands are available only for administrators.
	ChatAccessConfigAdmin ChatAccessLevel = 1
	// All commands are available only for administrators.
	ChatAccessAdminOnly ChatAccessLevel = 2
)

type Chat struct {
	ID               int             `db:"id" json:"id"`
	TgChatID         int64           `db:"tg_chat_id" json:"tg_chat_id"`
	UserName         *string         `db:"username" json:"username"`
	State            ChatState       `db:"state" json:"state"`
	DepartmentName   *string         `db:"department" json:"department"`
	GroupName        *string         `db:"group" json:"group"`
	DailySendingTime *string         `db:"daily_sending_time" json:"daily_sending_time"`
	PairSending      bool            `db:"pair_sending" json:"pair_sending"`
	Access           ChatAccessLevel `db:"access" json:"access"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at" json:"updated_at"`
}

func (c *Chat) IsPrivate() bool {
	return c.TgChatID > 0
}

// InsertChats inserts multiple chats into the database.
func (r *Repository) InsertChats(chats []Chat) error {
	log.Trace().Any("chats", chats).Msg("Inserting chats...")
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for _, chat := range chats {
		if _, err := tx.NamedExec(
			`INSERT INTO chats (
				tg_chat_id, username, state, department, "group", daily_sending_time, pair_sending, access
			) VALUES (
			 	:tg_chat_id, :username, :state, :department, :group, :daily_sending_time, :pair_sending, :access
			)`,
			chat,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert chat: %w", err)
		}
	}

	return tx.Commit()
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

func (r *Repository) UpdateChatTgChatID(id int, tgChatID int64) error {
	_, err := r.db.Exec(`UPDATE chats SET tg_chat_id = ?, updated_at = ? WHERE id = ?`, tgChatID, time.Now(), id)
	return err
}

func (r *Repository) GetChatCount() (int, error) {
	var count int
	if err := r.db.Get(&count, `SELECT COUNT(*) FROM chats`); err != nil {
		return 0, err
	}
	return count, nil
}

// GetPrivateChatCount returns the number of private chats.
//
// Private chat is a chat with Telegram chat ID greater than 0.
func (r *Repository) GetPrivateChatCount() (int, error) {
	var count int
	if err := r.db.Get(&count, `SELECT COUNT(*) FROM chats WHERE tg_chat_id > 0`); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) GetNewChatCount(dur time.Duration) (int, error) {
	var count int
	if err := r.db.Get(&count,
		`SELECT COUNT(*) FROM chats WHERE created_at > datetime('now', ?)`,
		fmt.Sprintf("-%d seconds", int(dur.Seconds())),
	); err != nil {
		return 0, err
	}
	return count, nil
}

// GetInactiveChatCount returns the number of inactive chats.
//
// Chat is inactive if it didn't use any commands for 48 hours,
// and have disabled all sendings or don't have group configured.
func (r *Repository) GetInactiveChatCount(dur time.Duration) (int, error) {
	var count int
	period := fmt.Sprintf("-%d seconds", int(dur.Seconds()))
	if err := r.db.Get(&count,
		`SELECT count(*) FROM (
			SELECT c.id, c.tg_chat_id, c.username, c."group", c.daily_sending_time, c.pair_sending, c.updated_at,
				count(ul.id) as count FROM chats c
			LEFT JOIN (
				SELECT * FROM update_logs WHERE created_at > datetime('now', ?)
			) ul ON c.id = ul.chat_id
			GROUP BY c.id
			HAVING count = 0 AND ("group" IS NULL OR "group" = '' OR daily_sending_time IS NULL AND pair_sending = 0)
		);`,
		period, period,
	); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) GetChatGroupedByDailySendingTime() (map[string]int, error) {
	var counts []struct {
		Time  string `db:"daily_sending_time"`
		Count int    `db:"count"`
	}
	if err := r.db.Select(&counts,
		`SELECT daily_sending_time, COUNT(*) as count FROM chats
		WHERE daily_sending_time IS NOT NULL
		GROUP BY daily_sending_time
		ORDER BY daily_sending_time ASC`,
	); err != nil {
		return nil, err
	}
	result := make(map[string]int)
	for _, count := range counts {
		result[count.Time] = count.Count
	}
	return result, nil
}

func (r *Repository) GetChatCountWithDailySendingEnabled() (int, error) {
	var count int
	if err := r.db.Get(&count,
		`SELECT COUNT(*) FROM chats WHERE daily_sending_time IS NOT NULL AND daily_sending_time != ''`,
	); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) GetChatCountWithPairSendingEnabled() (int, error) {
	var count int
	if err := r.db.Get(&count, `SELECT COUNT(*) FROM chats WHERE pair_sending = 1`); err != nil {
		return 0, err
	}
	return count, nil
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
	log.Trace().Str("timeStr", timeStr).Msg("Getting chats by daily sending time")
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
	log.Trace().Int("count", len(chats)).Any("chats", chats).Msg("Chats with pair sending enabled")
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

// DeleteAllChats deletes all chats from the database.
//
// This function is intended for testing purposes only.
func (r *Repository) DeleteAllChats() error {
	_, err := r.db.Exec(`DELETE FROM chats`)
	return err
}

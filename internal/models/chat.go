package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/pkg/utils"
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
	ID               int             `db:"id"`
	TgChatID         int64           `db:"tg_chat_id"`
	UserName         *string         `db:"username"`
	State            ChatState       `db:"state"`
	DepartmentName   *string         `db:"department"`
	GroupName        *string         `db:"group"`
	DailySendingTime *string         `db:"daily_sending_time"`
	PairSending      bool            `db:"pair_sending"`
	ChangeAlert      bool            `db:"update_notification"`
	Access           ChatAccessLevel `db:"access"`
	CreatedAt        time.Time       `db:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at"`
}

func (c *Chat) IsPrivate() bool {
	return c.TgChatID > 0
}

func (c *Chat) Update(db *sqlx.DB) error {
	_, err := db.NamedExec(
		`UPDATE chats
		SET username = :username,
			state = :state,
			department = :department,
			"group" = :group,
			daily_sending_time = :daily_sending_time,
			pair_sending = :pair_sending,
			update_notification = :update_notification,
			access = :access,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = :id`,
		c,
	)
	return err
}

// InsertChats inserts multiple chats into the database.
func InsertChats(db *sqlx.DB, chats []Chat) error {
	log.Trace().Any("chats", chats).Msg("Inserting chats...")
	tx, err := db.Beginx()
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

func CreateChat(db *sqlx.DB, tgChatID int64, username string) (int64, error) {
	res, err := db.Exec(
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
func CreateOrUpdateChat(db *sqlx.DB, tgChatID int64, username string) (*Chat, bool, error) {
	var chat Chat
	err := db.Get(&chat, `SELECT * FROM chats WHERE tg_chat_id = ?`, tgChatID)

	if err == sql.ErrNoRows {
		// Create new chat.
		log.Debug().Int64("tgChatID", tgChatID).Msg("Chat does not exist, creating new one...")
		id, err := CreateChat(db, tgChatID, username)
		if err != nil {
			return nil, true, fmt.Errorf("failed to create chat (%d, %s): %w", tgChatID, username, err)
		}

		chat, err := GetChat(db, id)
		return chat, true, err
	}

	if err != nil {
		log.Error().Err(err).Int64("tgChatID", tgChatID).Msg("Failed to get chat")
		return nil, false, fmt.Errorf("failed to get chat by Telegram chat ID (%d): %w", tgChatID, err)
	}

	if utils.DerefOrTypeDefault(chat.UserName) != username {
		// Update username.
		chat.UserName = &username
		if err := chat.Update(db); err != nil {
			err := fmt.Errorf("failed to update chat's username (%v -> %s): %w", chat.UserName, username, err)
			return nil, false, err
		}

		chat, err := GetChat(db, int64(chat.ID))
		return chat, false, err
	}

	// Return existing chat
	log.Trace().Int64("tgChatID", tgChatID).Msg("Chat already exists")
	return &chat, false, nil
}

func UpdateChatState(db *sqlx.DB, tgChatID int64, state ChatState) error {
	_, err := db.Exec(`UPDATE chats SET state = ?, updated_at = ? WHERE tg_chat_id = ?`, state, time.Now(), tgChatID)
	return err
}

func UpdateChatTgChatID(db *sqlx.DB, id int, tgChatID int64) error {
	_, err := db.Exec(`UPDATE chats SET tg_chat_id = ?, updated_at = ? WHERE id = ?`, tgChatID, time.Now(), id)
	return err
}

func GetChatCount(db *sqlx.DB) (int, error) {
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM chats`); err != nil {
		return 0, err
	}
	return count, nil
}

// GetPrivateChatCount returns the number of private chats.
//
// Private chat is a chat with Telegram chat ID greater than 0.
func GetPrivateChatCount(db *sqlx.DB) (int, error) {
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM chats WHERE tg_chat_id > 0`); err != nil {
		return 0, err
	}
	return count, nil
}

func GetNewChatCount(db *sqlx.DB, dur time.Duration) (int, error) {
	var count int
	if err := db.Get(&count,
		`SELECT COUNT(*) FROM chats WHERE created_at > datetime('now', ?)`,
		sqlPeriod(dur),
	); err != nil {
		return 0, err
	}
	return count, nil
}

func GetNewChatsGrouped(db *sqlx.DB, dur time.Duration) (map[string]int, error) {
	var data []struct {
		Group *string `db:"group"`
		Count int     `db:"count"`
	}
	if err := db.Select(
		&data,
		`SELECT "group", COUNT(*) AS count FROM chats WHERE created_at > datetime('now', ?) GROUP BY "group" ORDER BY "group"`,
		sqlPeriod(dur),
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	groupedChats := make(map[string]int)
	for _, item := range data {
		if item.Group == nil {
			item.Group = new(string) // Empty string
		}
		groupedChats[*item.Group] = item.Count
	}
	return groupedChats, nil
}

// GetInactiveChatCount returns the number of inactive chats.
//
// Chat is inactive if it didn't use any commands for 48 hours,
// and have disabled all sendings or don't have group configured.
func GetInactiveChatCount(db *sqlx.DB, dur time.Duration) (int, error) {
	var count int
	period := sqlPeriod(dur)
	if err := db.Get(&count,
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

func GetChatCountWithChangeAlertOn(db *sqlx.DB) (int, error) {
	var count int
	if err := db.Get(
		&count,
		`SELECT count(*) FROM chats WHERE update_notification= 1`,
		sqlPeriod(time.Hour*24), sqlPeriod(time.Hour*24),
	); err != nil {
		return 0, err
	}
	return count, nil
}

func GetChatGroupedByDailySendingTime(db *sqlx.DB) (map[string]int, error) {
	var counts []struct {
		Time  string `db:"daily_sending_time"`
		Count int    `db:"count"`
	}
	if err := db.Select(&counts,
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

func GetChatCountWithDailySendingEnabled(db *sqlx.DB) (int, error) {
	var count int
	if err := db.Get(&count,
		`SELECT COUNT(*) FROM chats WHERE daily_sending_time IS NOT NULL AND daily_sending_time != ''`,
	); err != nil {
		return 0, err
	}
	return count, nil
}

func GetChatCountWithPairSendingEnabled(db *sqlx.DB) (int, error) {
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM chats WHERE pair_sending = 1`); err != nil {
		return 0, err
	}
	return count, nil
}

func GetMonitoredGroups(db *sqlx.DB) ([]Group, error) {
	var groups []Group
	if err := db.Select(&groups, `
		SELECT DISTINCT g.* FROM groups g JOIN chats c ON g.group_name = c."group"
		WHERE "group" != '' AND "group" IS NOT NULL AND update_notification = 1
	`); err != nil {
		return nil, err
	}
	return groups, nil
}

func GetConfiguredGroupCount(db *sqlx.DB) (int, error) {
	var count int
	err := db.Get(&count, `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT "group" FROM chats
			WHERE "group" != '' AND "group" IS NOT NULL)
		`)
	return count, err
}

func GetAvgChatsPerGroup(db *sqlx.DB) (float32, error) {
	var avg float32
	err := db.Get(&avg, `
		SELECT AVG(chats) FROM (
			SELECT COUNT(*) AS chats FROM chats
			WHERE "group" != '' AND "group" IS NOT NULL
			GROUP BY "group"
		)
	`)
	return avg, err
}

func GetMedianChatsPerGroup(db *sqlx.DB) (float32, error) {
	var median float32
	err := db.Get(&median, `
		WITH ranked AS (
			SELECT
				value,
				ROW_NUMBER() OVER (ORDER BY value) AS row_num,
				COUNT(*) OVER () AS total_rows
			FROM (
				SELECT COUNT(*) AS value FROM chats
				WHERE "group" != '' AND "group" IS NOT NULL
				GROUP BY "group"
			)
		)
		SELECT AVG(value) AS median FROM ranked
		WHERE row_num IN ((total_rows + 1) / 2, (total_rows + 2) / 2);
	`)
	return median, err
}

func GetChat(db *sqlx.DB, id int64) (*Chat, error) {
	var chat Chat
	if err := db.Get(&chat, `SELECT * FROM chats WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return &chat, nil
}

func GetChatByTgChatID(db *sqlx.DB, tgChatID int64) (*Chat, error) {
	var chat Chat
	if err := db.Get(&chat, `SELECT * FROM chats WHERE tg_chat_id = ?`, tgChatID); err != nil {
		return nil, err
	}
	return &chat, nil
}

func GetChatByUserName(db *sqlx.DB, username string) (*Chat, error) {
	var chat Chat
	if err := db.Get(&chat, `SELECT * FROM chats WHERE LOWER(username) = ?`, strings.ToLower(username)); err != nil {
		return nil, err
	}
	return &chat, nil
}

func GetChats(db *sqlx.DB) ([]Chat, error) {
	var chats []Chat
	if err := db.Select(&chats, `SELECT * FROM chats`); err != nil {
		return nil, err
	}
	return chats, nil
}

func GetChatsByGroup(db *sqlx.DB, group string) ([]Chat, error) {
	var chats []Chat
	if err := db.Select(&chats, `SELECT * FROM chats WHERE "group" = ?`, group); err != nil {
		return nil, err
	}
	return chats, nil
}

func GetChatCountByGroup(db *sqlx.DB, group string) (int, error) {
	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM chats WHERE "group" = ?`, group); err != nil {
		return 0, err
	}
	return count, nil
}

func GetChatsByDailySendingTime(db *sqlx.DB, timeStr string) ([]Chat, error) {
	log.Trace().Str("timeStr", timeStr).Msg("Getting chats by daily sending time")
	var chats []Chat
	if err := db.Select(
		&chats, `SELECT * FROM chats WHERE "group" IS NOT NULL AND daily_sending_time = ?`, timeStr,
	); err != nil {
		return nil, err
	}
	return chats, nil
}

func GetChatsWithPairSendingEnabled(db *sqlx.DB) (chats []Chat, err error) {
	err = db.Select(&chats, `SELECT * FROM chats WHERE "group" IS NOT NULL AND pair_sending = 1`)
	log.Trace().Int("count", len(chats)).Any("chats", chats).Msg("Chats with pair sending enabled")
	return
}

func DeleteChat(db *sqlx.DB, id int) error {
	_, err := db.Exec(`DELETE FROM chats WHERE id = ?`, id)
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
func DeleteAllChats(db *sqlx.DB) error {
	_, err := db.Exec(`DELETE FROM chats`)
	return err
}

func sqlPeriod(dur time.Duration) string {
	sqlPeriod := fmt.Sprintf("-%d seconds", int(dur.Seconds()))
	return sqlPeriod
}

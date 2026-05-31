package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/pkg/refutil"
)

func NewChatRepository(db *sqlx.DB) ChatRepository { return &chatRepository{db} }

type ChatRepository interface {
	CreateOrUpdate(tgID model.ChatID, username model.UserName) (chat *model.Chat, created bool, err error)
	Create(tgID model.ChatID, username model.UserName) (int64, error)

	Get(id int) (*model.Chat, error)
	GetByChatID(model.ChatID) (*model.Chat, error)

	All() ([]model.Chat, error)
	AllNew(time.Duration) ([]model.Chat, error)
	AllByGroup(model.GroupName) ([]model.Chat, error)
	AllByWatchedGroup(model.GroupName) ([]model.Chat, error)
	AllByDailyTime(time string) ([]model.Chat, error)
	AllWithPairNotification() ([]model.Chat, error)
	AllWithChangeAlert() ([]model.Chat, error)

	WatchedGroups() ([]model.Group, error)

	Count() (int, error)
	CountPrivate() (int, error)
	CountUniqueConfiguredGroups() (int, error)
	CountDailySendingOn() (int, error)
	CountPairNotificationOn() (int, error)
	CountChangeAlertOn() (int, error)
	CountDarkModeOn() (int, error)
	CountDailyTimeGrouped() (map[string]int, error)
	CountNew(time.Duration) (int, error)
	CountInactive(time.Duration) (int, error)
	CountByGroup(model.GroupName) (int, error)

	GetAvgChatsPerGroup() (float32, error)
	GetMedianChatsPerGroup() (float32, error)

	Update(*model.Chat) error
	Delete(*model.Chat) error

	AddRecentTeacher(chatID, teacherID int) error
	RecentTeachers(chatID int) ([]model.RecentTeacher, error)
}

type chatRepository struct{ db *sqlx.DB }

func (r *chatRepository) CreateOrUpdate(tgChatID model.ChatID, username model.UserName) (chat *model.Chat, created bool, err error) {
	chat = new(model.Chat)
	err = r.db.Get(chat, `SELECT * FROM chats WHERE tg_chat_id = ?`, tgChatID)

	if err == sql.ErrNoRows {
		// Create new chat.
		log.Debug().Any("tgChatID", tgChatID).Msg("Chat does not exist, creating new one...")
		id, err := r.Create(tgChatID, username)
		if err != nil {
			return nil, true, fmt.Errorf("failed to create chat (%d, %s): %w", tgChatID, username, err)
		}

		chat, err := r.Get(int(id))
		return chat, true, err
	}
	if err != nil {
		log.Error().Err(err).Any("tgChatID", tgChatID).Msg("Failed to get chat")
		return nil, false, fmt.Errorf("failed to get chat by Telegram chat ID (%d): %w", tgChatID, err)
	}

	if refutil.DerefOrTypeDefault(chat.UserName) != username {
		// Update username.
		chat.UserName = &username
		if err := r.Update(chat); err != nil {
			err := fmt.Errorf("failed to update chat's username (%v -> %s): %w", chat.UserName, username, err)
			return nil, false, err
		}

		chat, err := r.Get(chat.ID)
		return chat, false, err
	}

	// Return existing chat
	log.Trace().Any("tgChatID", tgChatID).Msg("Chat already exists")
	return chat, false, nil
}

func (r *chatRepository) Create(tgID model.ChatID, username model.UserName) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO chats (tg_chat_id, username) VALUES (?,?)`, tgID, username)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *chatRepository) Get(id int) (*model.Chat, error) {
	var chat model.Chat
	if err := r.db.Get(&chat, `SELECT * FROM chats WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return &chat, nil
}
func (r *chatRepository) GetByChatID(tgID model.ChatID) (*model.Chat, error) {
	chat := model.Chat{}
	err := r.db.Get(&chat, `SELECT * FROM chats WHERE tg_chat_id = ?`, tgID)
	return &chat, err
}

func (r *chatRepository) All() ([]model.Chat, error) {
	var chats []model.Chat
	err := r.db.Select(&chats, `SELECT * FROM chats`)
	return chats, err
}
func (r *chatRepository) AllNew(dur time.Duration) ([]model.Chat, error) {
	var chats []model.Chat
	if err := r.db.Select(
		&chats,
		`SELECT * FROM chats WHERE created_at > datetime('now', ?)`,
		sqlPeriod(dur),
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return chats, nil
}
func (r *chatRepository) AllByGroup(name model.GroupName) ([]model.Chat, error) {
	var chats []model.Chat
	err := r.db.Select(&chats, `SELECT * FROM chats WHERE "group" = ?`, name)
	return chats, err
}
func (r *chatRepository) AllByWatchedGroup(name model.GroupName) ([]model.Chat, error) {
	var chats []model.Chat
	err := r.db.Select(&chats, `SELECT * FROM chats WHERE "group" = ? AND update_notification = 1`, name)
	return chats, err
}
func (r *chatRepository) AllByDailyTime(time string) ([]model.Chat, error) {
	var chats []model.Chat
	err := r.db.Select(&chats, `SELECT * FROM chats WHERE "group" IS NOT NULL AND daily_sending_time = ?`, time)
	return chats, err
}
func (r *chatRepository) AllWithPairNotification() ([]model.Chat, error) {
	var chats []model.Chat
	err := r.db.Select(&chats, `SELECT * FROM chats WHERE "group" IS NOT NULL AND pair_sending = 1`)
	return chats, err
}
func (r *chatRepository) AllWithChangeAlert() ([]model.Chat, error) {
	var chats []model.Chat
	err := r.db.Select(&chats, `
		SELECT * FROM chats WHERE "group" IS NOT NULL AND "group" != '' AND update_notification = 1
	`)
	return chats, err
}

func (r *chatRepository) WatchedGroups() ([]model.Group, error) {
	var groups []model.Group
	err := r.db.Select(&groups, `
		SELECT DISTINCT g.* FROM groups g JOIN chats c ON g.group_name = c."group"
		WHERE "group" != '' AND "group" IS NOT NULL AND update_notification = 1
	`)
	return groups, err
}

func (r *chatRepository) Count() (int, error) {
	var count int
	if err := r.db.Get(&count, `SELECT count(*) FROM chats`); err != nil {
		return 0, err
	}
	return count, nil
}
func (r *chatRepository) CountPrivate() (int, error) {
	var count int
	err := r.db.Get(&count, `SELECT COUNT(*) FROM chats WHERE tg_chat_id > 0`)
	return count, err
}
func (r *chatRepository) CountUniqueConfiguredGroups() (int, error) {
	var count int
	err := r.db.Get(&count, `
			SELECT COUNT(*) FROM (
				SELECT DISTINCT "group" FROM chats
				WHERE "group" != '' AND "group" IS NOT NULL
			)
		`)
	return count, err
}
func (r *chatRepository) CountDailySendingOn() (int, error) {
	var count int
	err := r.db.Get(&count, `
			SELECT COUNT(*) FROM chats
			WHERE daily_sending_time IS NOT NULL AND daily_sending_time != ''
		`)
	return count, err
}
func (r *chatRepository) CountPairNotificationOn() (int, error) {
	var count int
	err := r.db.Get(&count, `SELECT COUNT(*) FROM chats WHERE pair_sending = 1`)
	return count, err
}
func (r *chatRepository) CountChangeAlertOn() (int, error) {
	var count int
	err := r.db.Get(&count, `SELECT count(*) FROM chats WHERE update_notification= 1`)
	return count, err
}
func (r *chatRepository) CountDarkModeOn() (int, error) {
	var count int
	err := r.db.Get(&count, `SELECT count(*) FROM chats WHERE dark_mode = 1`)
	return count, err
}
func (r *chatRepository) CountDailyTimeGrouped() (map[string]int, error) {
	var counts []struct {
		Time  string `db:"daily_sending_time"`
		Count int    `db:"count"`
	}

	if err := r.db.Select(&counts, `
		SELECT daily_sending_time, COUNT(*) as count FROM chats
		WHERE daily_sending_time IS NOT NULL
		GROUP BY daily_sending_time
		ORDER BY daily_sending_time ASC
	`); err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for _, count := range counts {
		result[count.Time] = count.Count
	}
	return result, nil
}
func (r *chatRepository) CountNew(dur time.Duration) (int, error) {
	var count int
	err := r.db.Get(&count, `SELECT COUNT(*) FROM chats WHERE created_at > datetime('now', ?)`, sqlPeriod(dur))
	return count, err
}
func (r *chatRepository) CountInactive(dur time.Duration) (int, error) {
	var count int
	period := sqlPeriod(dur)
	err := r.db.Get(&count, `
			SELECT count(*) FROM (
				SELECT c.id, c.tg_chat_id, c.username, c."group", c.daily_sending_time, c.pair_sending, c.updated_at,
					count(ul.id) as count FROM chats c
				LEFT JOIN (
					SELECT * FROM update_logs WHERE created_at > datetime('now', ?)
				) ul ON c.id = ul.chat_id
				GROUP BY c.id
				HAVING count = 0 AND ("group" IS NULL OR "group" = '' OR daily_sending_time IS NULL AND pair_sending = 0)
			);
		`, period, period)
	return count, err
}
func (r *chatRepository) CountByGroup(name model.GroupName) (int, error) {
	var count int
	if err := r.db.Get(&count, `SELECT COUNT(*) FROM chats WHERE "group" = ?`, name); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *chatRepository) GetAvgChatsPerGroup() (float32, error) {
	var avg float32
	err := r.db.Get(&avg, `
		SELECT AVG(chats) FROM (
			SELECT COUNT(*) AS chats FROM chats
			WHERE "group" != '' AND "group" IS NOT NULL
			GROUP BY "group"
		)
	`)
	return avg, err
}
func (r *chatRepository) GetMedianChatsPerGroup() (float32, error) {
	var median float32
	err := r.db.Get(&median, `
		WITH ranked AS (
			SELECT
				value,
				row_number() OVER (ORDER BY value) AS row_num,
				count(*) OVER () AS total_rows
			FROM (
				SELECT count(*) AS value FROM chats
				WHERE "group" != '' AND "group" IS NOT NULL
				GROUP BY "group"
			)
		)
		SELECT avg(value) AS median FROM ranked
		WHERE row_num IN ((total_rows + 1) / 2, (total_rows + 2) / 2);
	`)
	return median, err
}

func (r *chatRepository) Update(chat *model.Chat) error {
	_, err := r.db.NamedExec(`
			UPDATE chats
			SET username = :username,
				state = :state,
				department = :department,
				"group" = :group,
				daily_sending_time = :daily_sending_time,
				pair_sending = :pair_sending,
				update_notification = :update_notification,
				access = :access,
				dark_mode = :dark_mode,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = :id
		`, chat)
	return err
}

func (r *chatRepository) Delete(chat *model.Chat) error {
	_, err := r.db.Exec(`DELETE FROM chats WHERE id = ?`, chat.ID)
	return err
}

func (r *chatRepository) AddRecentTeacher(chatID, teacherID int) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	if _, err := tx.Exec(
		`DELETE FROM recent_teachers WHERE chat_id = ? AND teacher_id = ?`, chatID, teacherID,
	); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete same recent teachers (%d) of chat (%d): %w", teacherID, chatID, err)
	}

	rt, err := r.RecentTeachers(chatID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to get recent teachers (%d) of chat (%d): %w", teacherID, chatID, err)
	}

	if len(rt) >= 4 {
		if _, err := tx.NamedExec(
			`DELETE FROM recent_teachers WHERE chat_id = :chat_id AND teacher_id = :teacher_id`, rt[0],
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete oldest recent teacher (%d) of chat (%d): %w",
				rt[0].TeacherID, chatID, err)
		}
	}

	if _, err := tx.Exec(
		`INSERT INTO recent_teachers (chat_id, teacher_id) VALUES (?,?)`, chatID, teacherID,
	); err != nil {
		return fmt.Errorf("failed to insert recent teacher: %w", err)
	}

	return tx.Commit()
}
func (r *chatRepository) RecentTeachers(chatID int) ([]model.RecentTeacher, error) {
	var rt []model.RecentTeacher
	err := r.db.Select(&rt, `SELECT * FROM recent_teachers WHERE chat_id = ? ORDER BY created_at ASC`, chatID)
	return rt, err
}

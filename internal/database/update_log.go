package database

import (
	"time"
)

type UpdateLog struct {
	ID           int       `db:"id"`
	ChatID       int       `db:"chat_id"` // Chat.ID
	Kind         string    `db:"kind"`    // Variants: "message", "callback_query"
	MessageID    int       `db:"message_id"`
	Data         string    `db:"data"`
	HandlingTime int       `db:"handling_time"` // Milliseconds
	Error        string    `db:"error"`
	CreatedAt    time.Time `db:"created_at"`
}

func (ul *UpdateLog) IsOk() bool {
	return ul.Error == ""
}

func (r *Repository) GetUpdateLogByChatID(ID int) ([]UpdateLog, error) {
	var logs []UpdateLog
	err := r.db.Select(&logs, "SELECT * FROM update_logs WHERE chat_id = ?", ID)
	return logs, err
}

func (r *Repository) GetRecentChatUpdateLogs(ID int, dur time.Duration) ([]UpdateLog, error) {
	var logs []UpdateLog
	err := r.db.Select(&logs, "SELECT * FROM update_logs WHERE chat_id = ?", ID, time.Now().Add(-dur))
	return logs, err
}

func (r *Repository) InsertUpdateLog(log *UpdateLog) error {
	_, err := r.db.NamedExec(
		`INSERT INTO update_logs (chat_id, kind, message_id, data, handling_time, error)
		VALUES (:chat_id, :kind, :message_id, :data, :handling_time, :error)`,
		log,
	)
	return err
}

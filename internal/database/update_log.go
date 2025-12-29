package database

import (
	"fmt"
	"time"
)

type UpdateLog struct {
	ID           int       `db:"id"`
	ChatID       int       `db:"chat_id"` // Chat.ID
	Kind         string    `db:"kind"`    // Variants: "message", "callback_query"
	MessageID    int       `db:"message_id"`
	Data         string    `db:"data"`
	HandlingTime int       `db:"handling_time"` // Milliseconds
	Error        *string   `db:"error"`
	CreatedAt    time.Time `db:"created_at"`
}

func (ul *UpdateLog) IsOk() bool {
	return ul.Error == nil || *ul.Error == ""
}

func (r *Repository) GetUpdateLogByChatID(ID int) ([]UpdateLog, error) {
	var logs []UpdateLog
	err := r.db.Select(&logs, "SELECT * FROM update_logs WHERE chat_id = ?", ID)
	return logs, err
}

func (r *Repository) GetUpdateLogsByPeriod(start, end time.Time) ([]UpdateLog, error) {
	var logs []UpdateLog
	err := r.db.Select(&logs, "SELECT * FROM update_logs WHERE created_at >= ? AND created_at <= ?", start, end)
	return logs, err
}

func (r *Repository) GetRecentChatUpdateLogs(ID int, dur time.Duration) ([]UpdateLog, error) {
	var logs []UpdateLog
	err := r.db.Select(&logs, "SELECT * FROM update_logs WHERE chat_id = ? AND created_at > ?", ID, time.Now().Add(-dur))
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

type DistributionElement struct {
	Name  string `db:"name"`
	Value int    `db:"value"`
}

func (r *Repository) GetDistribution(dataKind, periodKind string, dur time.Duration) ([]DistributionElement, error) {
	nameQuery :=
		`CASE strftime('%w', created_at)
			WHEN '0' THEN '7'
			WHEN '1' THEN '1'
			WHEN '2' THEN '2'
			WHEN '3' THEN '3'
			WHEN '4' THEN '4'
			WHEN '5' THEN '5'
			WHEN '6' THEN '6'
		END`
	switch periodKind {
	case "day":
		nameQuery = "strftime('%H:00', created_at)"
	// case "week":
	// 	// Default
	case "month":
		nameQuery = "strftime('%y-%m-%d', created_at)"
	case "year":
		nameQuery = "strftime('%y-%m', created_at)"
	}

	period := fmt.Sprintf("-%d seconds", int(dur.Seconds()))

	switch dataKind {
	case "activity":
		var elements []DistributionElement
		if err := r.db.Select(
			&elements,
			`SELECT `+nameQuery+` AS name,
				COUNT(*) AS value
			FROM update_logs
			WHERE created_at > datetime('now', ?)
			GROUP BY name
			ORDER BY name ASC`,
			period,
		); err != nil {
			return nil, err
		}

		return elements, nil
	default:
		return nil, nil
	}
}

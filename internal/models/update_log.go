package models

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
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

func GetUpdateLogByChatID(db *sqlx.DB, ID int) ([]UpdateLog, error) {
	var logs []UpdateLog
	err := db.Select(&logs, "SELECT * FROM update_logs WHERE chat_id = ?", ID)
	return logs, err
}

func GetUpdateLogsByPeriod(db *sqlx.DB, start, end time.Time) ([]UpdateLog, error) {
	var logs []UpdateLog
	err := db.Select(&logs, "SELECT * FROM update_logs WHERE created_at >= ? AND created_at <= ?", start, end)
	return logs, err
}

func GetRecentChatUpdateLogs(db *sqlx.DB, ID int, dur time.Duration) ([]UpdateLog, error) {
	var logs []UpdateLog
	err := db.Select(&logs, "SELECT * FROM update_logs WHERE chat_id = ? AND created_at > ?", ID, time.Now().Add(-dur))
	return logs, err
}

func InsertUpdateLog(db *sqlx.DB, log *UpdateLog) error {
	_, err := db.NamedExec(
		`INSERT INTO update_logs (chat_id, kind, message_id, data, handling_time, error)
		VALUES (:chat_id, :kind, :message_id, :data, :handling_time, :error)`,
		log,
	)
	return err
}

type DistItem struct {
	Name  string `DB:"name"`
	Value int    `DB:"value"`
}

func GetDist(db *sqlx.DB, dataKind, periodKind string, dur time.Duration) ([]DistItem, error) {
	nameQuery :=
		`CASE strftime('%w', datetime(created_at, 'localtime'))
			WHEN '0' THEN '7'
			WHEN '1' THEN '1'
			WHEN '2' THEN '2'
			WHEN '3' THEN '3'
			WHEN '4' THEN '4'
			WHEN '5' THEN '5'
			WHEN '6' THEN '6'
		END`
	switch periodKind {
	case "h": //Hours for day
		nameQuery = "strftime('%H:00', datetime(created_at, 'localtime'))"
	case "d": // Days of month
		nameQuery = "strftime('%y-%m-%d', datetime(created_at, 'localtime'))"
	case "m": // Months of year
		nameQuery = "strftime('%y-%m', datetime(created_at, 'localtime'))"
		// Default: Week days
	}

	period := fmt.Sprintf("-%d seconds", int(dur.Seconds()))

	// TODO: Come up with variants for dataKind.
	switch dataKind {
	case "a":
		var items []DistItem
		if err := db.Select(
			&items,
			`SELECT `+nameQuery+` AS name,
				COUNT(*) AS value
			FROM update_logs
			WHERE created_at > datetime('now', 'localtime', ?)
			GROUP BY name
			ORDER BY name ASC`,
			period,
		); err != nil {
			return nil, err
		}

		return items, nil
	default:
		log.Warn().Str("dataKind", dataKind).Msg("Unsupported dataKind")
		return nil, nil
	}
}

package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/model"
)

func NewLogRepository(db *sqlx.DB) LogRepository { return &logRepository{db} }

type LogRepository interface {
	LogSending(model.SendingLog) error
	GetSendingLogs(model.SendingLogKind, time.Duration) ([]model.SendingLog, error)
	CountSendingLogs(model.SendingLogKind, time.Duration) (total, ok, fails int, err error)

	LogUpdate(*model.UpdateLog) error
	UpdateLogsByChatID(int) ([]model.UpdateLog, error)
	UpdateLogsByPeriod(start, end time.Time) ([]model.UpdateLog, error)
	RecentChatUpdateLogs(chatID int, dur time.Duration) ([]model.UpdateLog, error)
	UpdateDist(dataKind, periodKind string, dur time.Duration) ([]DistItem, error)
}

type DistItem struct {
	Name  string `db:"name"`
	Value int    `db:"value"`
}

type logRepository struct{ db *sqlx.DB }

func (r *logRepository) LogSending(log model.SendingLog) error {
	if _, err := r.db.NamedExec(`
			INSERT INTO sending_logs (kind, chats, groups, elapsed, fails, errors)
			VALUES (:kind, :chats, :groups, :elapsed, :fails, :errors)
		`, log); err != nil {

		return fmt.Errorf("failed to insert sending log: %w", err)
	}
	return nil
}
func (r *logRepository) GetSendingLogs(kind model.SendingLogKind, dur time.Duration) ([]model.SendingLog, error) {
	query := `SELECT * FROM sending_logs WHERE created_at >= ?`
	switch kind {
	case model.SendingLogDaily, model.SendingLogPair, model.SendingLogChange:
		query += fmt.Sprintf(` AND kind = '%s'`, kind)
	}

	var logs []model.SendingLog
	if err := r.db.Select(&logs, query, time.Now().Add(-dur)); err != nil {
		return nil, fmt.Errorf("failed to get sending logs for duration: %w", err)
	}
	return logs, nil
}
func (r *logRepository) CountSendingLogs(kind model.SendingLogKind, dur time.Duration) (total, ok, fails int, err error) {
	query := `
		SELECT sum(chats) AS total, sum(fails) AS fails
		FROM sending_logs
		WHERE created_at >= date('now', 'localtime', ?)
	`
	switch kind {
	case model.SendingLogDaily, model.SendingLogPair, model.SendingLogChange:
		query += fmt.Sprintf(` AND kidn = '%s'`, kind)
	}

	var dest struct {
		Total sql.NullInt32 `db:"total"`
		Fails sql.NullInt32 `db:"fails"`
	}
	if err := r.db.Get(&dest, query, sqlPeriod(dur)); err != nil {
		return -1, -1, -1, fmt.Errorf("failed to count sending logs: %w", err)
	}
	return int(dest.Total.Int32), int(dest.Total.Int32 - dest.Fails.Int32), int(dest.Fails.Int32), nil
}

func (r *logRepository) LogUpdate(log *model.UpdateLog) error {
	_, err := r.db.NamedExec(`
			INSERT INTO update_logs (chat_id, kind, message_id, data, handling_time, error)
			VALUES (:chat_id, :kind, :message_id, :data, :handling_time, :error)
		`, log)
	return err
}
func (r *logRepository) UpdateLogsByChatID(id int) ([]model.UpdateLog, error) {
	var logs []model.UpdateLog
	err := r.db.Select(&logs, "SELECT * FROM update_logs WHERE chat_id = ?", id)
	return logs, err
}
func (r *logRepository) UpdateLogsByPeriod(start, end time.Time) ([]model.UpdateLog, error) {
	var logs []model.UpdateLog
	err := r.db.Select(&logs, "SELECT * FROM update_logs WHERE created_at >= ? AND created_at <= ?", start, end)
	return logs, err
}
func (r *logRepository) RecentChatUpdateLogs(chatID int, dur time.Duration) ([]model.UpdateLog, error) {
	var logs []model.UpdateLog
	err := r.db.Select(&logs,
		`SELECT * FROM update_logs WHERE chat_id = ? AND created_at > ?`,
		chatID, time.Now().Add(-dur),
	)
	return logs, err
}
func (r *logRepository) UpdateDist(dataKind, periodKind string, dur time.Duration) ([]DistItem, error) {
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
	case "h": // Hours of day
		nameQuery = "strftime('%H:00', datetime(created_at, 'localtime'))"
	case "d": // Days of month
		nameQuery = "strftime('%y-%m-%d', datetime(created_at, 'localtime'))"
	case "m": // Months of year
		nameQuery = "strftime('%y-%m', datetime(created_at, 'localtime'))"
		// Default: Week days
	}

	// TODO: Come up with variants for dataKind.
	switch dataKind {
	case "a":
		var items []DistItem
		if err := r.db.Select(
			&items,
			`SELECT `+nameQuery+` AS name,
				COUNT(*) AS value
			FROM update_logs
			WHERE created_at > datetime('now', 'localtime', ?)
			GROUP BY name
			ORDER BY name ASC`,
			sqlPeriod(dur),
		); err != nil {
			return nil, err
		}

		return items, nil
	default:
		log.Warn().Str("dataKind", dataKind).Msg("Unsupported dataKind")
		return nil, nil
	}
}

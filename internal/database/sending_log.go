package database

import (
	"fmt"
	"time"
)

type SendingLog struct {
	ID     int            `db:"id"`
	Kind   SendingLogKind `db:"kind"`
	Chats  int            `db:"chats"`
	Groups int            `db:"groups"`
	// milliseconds
	Elapsed   int       `db:"elapsed"`
	Fails     int       `db:"fails"`
	Errors    string    `db:"errors"`
	CreatedAt time.Time `db:"created_at"`
}

type SendingLogKind string

const (
	AnySendingLog   SendingLogKind = ""
	DailySendingLog SendingLogKind = "daily"
	PairSendingLog  SendingLogKind = "pair"
)

func (r *Repository) InsertSendingLog(log SendingLog) error {
	if _, err := r.db.NamedExec(
		`INSERT INTO sending_logs (kind, chats, groups, elapsed, fails, errors)
	VALUES (:kind, :chats, :groups, :elapsed, :fails, :errors)`,
		log,
	); err != nil {
		return fmt.Errorf("failed to insert sending log: %w", err)
	}
	return nil
}

func (r *Repository) GetSendingLogsForPeriod(kind SendingLogKind, start, end time.Time) ([]SendingLog, error) {
	query := ""
	switch kind {
	case AnySendingLog:
		query = "SELECT * FROM sending_logs WHERE datetime(created_at, 'localtime') BETWEEN ? AND ?"
	case DailySendingLog:
		query = "SELECT * FROM sending_logs WHERE type = 'daily' AND datetime(created_at, 'localtime') BETWEEN ? AND ?"
	case PairSendingLog:
		query = "SELECT * FROM sending_logs WHERE type = 'pair' AND datetime(created_at, 'localtime') BETWEEN ? AND ?"
	}
	var logs []SendingLog
	if err := r.db.Select(&logs, query, start, end); err != nil {
		return nil, fmt.Errorf("failed to get sending logs for period: %w", err)
	}
	return logs, nil
}

func (r *Repository) GetSendingLogs(kind SendingLogKind, dur time.Duration) ([]SendingLog, error) {
	query := ""
	switch kind {
	case AnySendingLog:
		query = "SELECT * FROM sending_logs WHERE created_at >= ?"
	case DailySendingLog:
		query = "SELECT * FROM sending_logs WHERE type = 'daily' AND created_at >= ?"
	case PairSendingLog:
		query = "SELECT * FROM sending_logs WHERE type = 'pair' AND created_at >= ?"
	}
	var logs []SendingLog
	if err := r.db.Select(&logs, query, time.Now().Add(-dur)); err != nil {
		return nil, fmt.Errorf("failed to get sending logs for duration: %w", err)
	}
	return logs, nil
}

func (r *Repository) GetDailySendingLogs(dur time.Duration) ([]SendingLog, error) {
	return r.GetSendingLogs(DailySendingLog, dur)
}

func (r *Repository) GetPairSendingLogsForPair(dur time.Duration) ([]SendingLog, error) {
	return r.GetSendingLogs(PairSendingLog, dur)
}

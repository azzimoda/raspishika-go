package models

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
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

func InsertSendingLog(db *sqlx.DB, log SendingLog) error {
	if _, err := db.NamedExec(
		`INSERT INTO sending_logs (kind, chats, groups, elapsed, fails, errors)
	VALUES (:kind, :chats, :groups, :elapsed, :fails, :errors)`,
		log,
	); err != nil {
		return fmt.Errorf("failed to insert sending log: %w", err)
	}
	return nil
}

func GetSendingLogs(db *sqlx.DB, kind SendingLogKind, dur time.Duration) ([]SendingLog, error) {
	query := ""
	switch kind {
	case AnySendingLog:
		query = "SELECT * FROM sending_logs WHERE created_at >= ?"
	case DailySendingLog:
		query = "SELECT * FROM sending_logs WHERE kind = 'daily' AND created_at >= ?"
	case PairSendingLog:
		query = "SELECT * FROM sending_logs WHERE kind = 'pair' AND created_at >= ?"
	}
	var logs []SendingLog
	if err := db.Select(&logs, query, time.Now().Add(-dur)); err != nil {
		return nil, fmt.Errorf("failed to get sending logs for duration: %w", err)
	}
	return logs, nil
}

func GetSendingLogsCount(
	db *sqlx.DB,
	kind SendingLogKind,
	dur time.Duration,
) (total int, ok int, fails int, err error) {
	query := ""
	switch kind {
	case AnySendingLog:
		query = "SELECT SUM(CHATS) AS total, SUM(fails) AS fails FROM sending_logs WHERE created_at >= ?"
	case DailySendingLog:
		query = "SELECT SUM(CHATS) AS total, SUM(fails) AS fails FROM sending_logs WHERE kind = 'daily' AND created_at >= ?"
	case PairSendingLog:
		query = "SELECT SUM(CHATS) AS total, SUM(fails) AS fails FROM sending_logs WHERE kind = 'pair' AND created_at >= ?"
	}
	var count struct {
		Total int `db:"total"`
		Fails int `db:"fails"`
	}
	if err := db.Get(&count, query, time.Now().Add(-dur)); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get sending logs count for duration: %w", err)
	}
	return count.Total, count.Total - count.Fails, count.Fails, nil
}

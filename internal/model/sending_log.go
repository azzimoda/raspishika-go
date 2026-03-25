package model

import "time"

type SendingLog struct {
	ID        int            `db:"id"`
	Kind      SendingLogKind `db:"kind"`
	Chats     int            `db:"chats"`
	Groups    int            `db:"groups"`
	Elapsed   int            `db:"elapsed"` // milliseconds
	Fails     int            `db:"fails"`
	Errors    string         `db:"errors"`
	CreatedAt time.Time      `db:"created_at"`
}

type SendingLogKind string

const (
	SendingLogAny    SendingLogKind = ""
	SendingLogDaily  SendingLogKind = "daily"
	SendingLogPair   SendingLogKind = "pair"
	SendingLogChange SendingLogKind = "update"
)

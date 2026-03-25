package model

import (
	"fmt"
	"time"
)

type ChatID int64

func (i ChatID) Int64() int64 { return int64(i) }

func (i ChatID) IsPrivate() bool { return i > 0 }

type UserName string

func (n UserName) String() string { return string(n) }

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
	TgChatID         ChatID          `db:"tg_chat_id"`
	UserName         *UserName       `db:"username"` // TODO: Make this field NOT NULL — use just empty string·
	State            ChatState       `db:"state"`
	DepartmentName   *DepartmentName `db:"department"`
	GroupName        *GroupName      `db:"group"`
	DailySendingTime *string         `db:"daily_sending_time"`
	PairSending      bool            `db:"pair_sending"`
	ChangeAlert      bool            `db:"update_notification"`
	Access           ChatAccessLevel `db:"access"`
	DarkMode         bool            `db:"dark_mode"`
	CreatedAt        time.Time       `db:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at"`
}

func (c *Chat) IsPrivate() bool { return c.TgChatID.IsPrivate() }

func sqlPeriod(dur time.Duration) string {
	sqlPeriod := fmt.Sprintf("-%d seconds", int(dur.Seconds()))
	return sqlPeriod
}

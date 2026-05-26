package model

import (
	"fmt"
	"time"

	"github.com/azzimoda/raspishika-go/internal/config"
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
	UserName         *UserName       `db:"username"` // TODO: Make this field NOT NULL — use just empty string
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

// GetState returns actual state of the chat.
//
// If chat's state is not Defeult and chat state TTL is expired, returns (ChatStateDefault, true).
// Otherwise returns actual state and false.
func (c *Chat) GetState() (state ChatState, expired bool) {
	if c.State != ChatStateDefault && time.Since(c.UpdatedAt) >= config.ChatStateTTL() {
		c.State = ChatStateDefault
		return ChatStateDefault, true
	}
	return c.State, false
}

// WithState updates chat's state and returns reference to this chat.
func (c *Chat) WithState(state ChatState) *Chat {
	c.State = state
	return c
}

func sqlPeriod(dur time.Duration) string {
	sqlPeriod := fmt.Sprintf("-%d seconds", int(dur.Seconds()))
	return sqlPeriod
}

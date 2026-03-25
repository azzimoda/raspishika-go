package repository

import "github.com/jmoiron/sqlx"

func NewContainer(db *sqlx.DB) *Container {
	return &Container{
		Chat:     NewChatRepository(db),
		Group:    NewGroupRepository(db),
		Schedule: NewScheduleRepository(db),
		Log:      NewLogRepository(db),
	}
}

type Container struct {
	Chat     ChatRepository
	Group    GroupRepository
	Schedule ScheduleRepository
	Log      LogRepository
}

package repository

import (
	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/jmoiron/sqlx"
)

func NewScheduleRepository(db *sqlx.DB) ScheduleRepository { return &scheduleRepository{db} }

type ScheduleRepository interface {
	Insert(*model.Schedule) error
	InsertOrUpdate(*model.Schedule) error
	GetByKey(string) (*model.Schedule, error)
	Update(*model.Schedule) error
}

type scheduleRepository struct{ db *sqlx.DB }

func (r *scheduleRepository) Insert(schedule *model.Schedule) error {
	_, err := r.db.NamedExec(`
		INSERT INTO schedules (group_id, teacher_id, data)
		VALUES (:group_id, :teacher_id, :data)
	`, schedule)
	return err
}
func (r *scheduleRepository) InsertOrUpdate(schedule *model.Schedule) error {
	_, err := r.db.NamedExec(`
		INSERT INTO schedules (cache_key, data)
		VALUES (:cache_key, :data)
		ON CONFLICT (cache_key) DO UPDATE SET data = :data, updated_at = CURRENT_TIMESTAMP
	`, schedule)
	return err
}
func (r *scheduleRepository) GetByKey(key string) (*model.Schedule, error) {
	var schedule model.Schedule
	err := r.db.Get(&schedule, `SELECT * FROM schedules WHERE cache_key = ?`, key)
	return &schedule, err
}
func (r *scheduleRepository) Update(schedule *model.Schedule) error {
	_, err := r.db.NamedExec(`
		UPDATE schedules
		SET data = :data, updated_at = CURRENT_TIMESTAMP
		WHERE cache_key = :cache_key
	`, schedule)
	return err
}

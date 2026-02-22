package models

import (
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func NewSchedule(cacheKey string, rawSchedule RawSchedule) *Schedule {
	jsonData, err := json.Marshal(rawSchedule)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal raw schedule")
		return nil
	}
	return &Schedule{CacheKey: cacheKey, Data: string(jsonData)}
}

func GetSchedule(db *sqlx.DB, key string) (*Schedule, error) {
	var schedule Schedule
	err := db.Get(&schedule, `SELECT * FROM schedules WHERE cache_key = ?`, key)
	return &schedule, err
}

type Schedule struct {
	ID        int       `db:"id"`
	CacheKey  string    `db:"cache_key"`
	Data      string    `db:"data"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (s *Schedule) IsActual(ttl time.Duration) bool { return s.UpdatedAt.Add(ttl).After(time.Now()) }

func (s *Schedule) Unmarshal() (*RawSchedule, error) {
	var rawSchedule RawSchedule
	err := json.Unmarshal([]byte(s.Data), &rawSchedule)
	return &rawSchedule, err
}

func (s *Schedule) Insert(db *sqlx.DB) error {
	_, err := db.NamedExec(`
		INSERT INTO schedules (group_id, teacher_id, data)
		VALUES (:group_id, :teacher_id, :data)
	`, s)
	return err
}

func (s *Schedule) Update(db *sqlx.DB) error {
	_, err := db.NamedExec(`
		UPDATE schedules
		SET data = :data, updated_at = CURRENT_TIMESTAMP
		WHERE cache_key = :cache_key
	`, s)
	return err
}

func (s Schedule) InsertOrUpdate(db *sqlx.DB) error {
	log.Trace().Any("CacheKey", s.CacheKey).Msg("Inserting/Updating...")
	_, err := db.NamedExec(`
		INSERT INTO schedules (cache_key, data)
		VALUES (:cache_key, :data)
		ON CONFLICT (cache_key) DO UPDATE SET data = :data, updated_at = CURRENT_TIMESTAMP
	`, s)
	return err
}

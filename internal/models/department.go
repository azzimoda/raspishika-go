package models

import (
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

func GetDepartments(db *sqlx.DB) (departments []Department, err error) {
	err = db.Select(&departments, `SELECT id, name, url, created_at, updated_at FROM departments`)
	return departments, err
}

type Department struct {
	ID        int       `db:"id"`
	Name      string    `db:"name"`
	URL       string    `db:"url"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (d *Department) IsActual(ttl time.Duration) bool { return d.UpdatedAt.Add(ttl).After(time.Now()) }

func (d *Department) Insert(db *sqlx.DB) error {
	_, err := db.NamedExec(`
		INSERT INTO departments (name, url, created_at, updated_at)
		VALUES (:name, :url, :created_at, :updated_at)
	`, d)
	return err
}

func (d *Department) Update(db *sqlx.DB) error {
	_, err := db.NamedExec(`
		UPDATE departments
		SET name = :name, url = :url, updated_at = CURRENT_TIMESTAMP
		WHERE id = :id
	`, d)
	return err
}

func (d *Department) InsertOrUpdate(db *sqlx.DB) error {
	log.Trace().Any("department", d).Msg("Inserting...")
	_, err := db.NamedExec(`
		INSERT INTO departments (name, url)
		VALUES (:name, :url)
		ON CONFLICT (name) DO UPDATE SET url = :url, updated_at = CURRENT_TIMESTAMP
	`, d)
	return err
}

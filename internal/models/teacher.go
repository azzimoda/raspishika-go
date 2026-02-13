package models

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type Teacher struct {
	ID        int       `db:"id" json:"id"`
	TeacherID string    `db:"teacher_id" json:"teacher_id"`
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func GetTeacherByTeacherID(db *sqlx.DB, teacherID string) (*Teacher, error) {
	var t Teacher
	err := db.Get(&t, `SELECT * FROM teachers WHERE teacher_id = ?`, teacherID)
	return &t, err
}

func GetTeachers(db *sqlx.DB) (teachers []Teacher, err error) {
	err = db.Select(&teachers, "SELECT * FROM teachers")
	return
}

func GetTeacherByChatID(db *sqlx.DB, ID int) ([]Teacher, error) {
	var teachers []Teacher
	err := db.Select(&teachers,
		`SELECT t.id, t.teacher_id, t.name, t.created_at, t.updated_at
		FROM recent_teachers rt JOIN teachers t ON rt.teacher_id = t.id
		WHERE rt.chat_id = ?`,
		ID,
	)
	return teachers, err
}

func UpdateTeachers(db *sqlx.DB, teachers []Teacher) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	for _, t := range teachers {
		var _t Teacher
		if err := tx.Get(&_t, `SELECT * FROM teachers WHERE teacher_id = ?`, t.TeacherID); err != nil {
			// Insert new.
			_, err = tx.NamedExec(`INSERT INTO teachers (teacher_id, name) VALUES (:teacher_id, :name)`, t)
			if err != nil {
				tx.Rollback()
				return err
			}
		}

		// Update existing.
		t.UpdatedAt = time.Now()
		_, err = tx.NamedExec(
			`UPDATE teachers SET name = :name, updated_at = :updated_at WHERE teacher_id = :teacher_id`,
			t)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

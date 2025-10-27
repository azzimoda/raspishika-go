package database

import (
	"time"
)

type Teacher struct {
	ID        int64     `db:"id" json:"id"`
	TeacherID string    `db:"teacher_id" json:"teacher_id"`
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

func (r *Repository) GetTeachers() (teachers []Teacher, err error) {
	err = r.db.Select(&teachers, "SELECT id, teacher_id, name FROM teachers")
	return
}

func (r *Repository) UpdateTeachers(teachers []Teacher) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return err
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

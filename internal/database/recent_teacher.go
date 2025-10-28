package database

import (
	"fmt"
	"time"
)

type RecentTeacher struct {
	ID        int64     `db:"id"`
	ChatID    int64     `db:"chat_id"`
	TeacherID int64     `db:"teacher_id"`
	CreatedAt time.Time `db:"created_at"`
}

func (r *Repository) GetChatRecentTeachers(chatID int64) ([]RecentTeacher, error) {
	var rt []RecentTeacher
	err := r.db.Select(&rt, `SELECT * FROM recent_teachers WHERE chat_id = ? ORDER BY created_at ASC`, chatID)
	return rt, err
}

// AddRecentTeacher inserts new recent teacher record to table recent_teachers,
// ensuring that there is less than or equal to 6 records for the given chatID.
//
// Args:
// chatID is ID field of struct Chat;
// teacherID is ID field of struct Teacher.
func (r *Repository) AddChatRecentTeacher(chatID, teacherID int64) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	if _, err := tx.Exec(
		`DELETE FROM recent_teachers WHERE chat_id = ? AND teacher_id = ?`, chatID, teacherID,
	); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete same recent teachers (%d) of chat (%d): %w", teacherID, chatID, err)
	}

	rt, err := r.GetChatRecentTeachers(chatID)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to get recent teachers (%d) of chat (%d): %w", teacherID, chatID, err)
	}

	if len(rt) >= 4 {
		if _, err := tx.NamedExec(
			`DELETE FROM recent_teachers WHERE chat_id = :chat_id AND teacher_id = :teacher_id`, rt[0],
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete oldest recent teacher (%d) of chat (%d): %w",
				rt[0].TeacherID, chatID, err)
		}
	}

	if _, err := tx.Exec(
		`INSERT INTO recent_teachers (chat_id, teacher_id) VALUES (?,?)`, chatID, teacherID,
	); err != nil {
		return fmt.Errorf("failed to insert recent teacher: %w", err)
	}

	return tx.Commit()
}

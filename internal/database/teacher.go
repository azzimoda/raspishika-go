package database

type Teacher struct {
	ID        int64  `db:"id" json:"id"`
	TeacherID string `db:"teacher_id" json:"teacher_id"`
	Name      string `db:"name" json:"name"`
}

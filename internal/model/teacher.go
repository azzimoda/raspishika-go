package model

import "time"

type TeacherID string

func (i TeacherID) String() string { return string(i) }

type TeacherName string

func (n TeacherName) String() string { return string(n) }

type Teacher struct {
	ID        int         `db:"id"         json:"id"`
	TeacherID TeacherID   `db:"teacher_id" json:"teacher_id"`
	Name      TeacherName `db:"name"       json:"name"`
	CreatedAt time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt time.Time   `db:"updated_at" json:"updated_at"`
}

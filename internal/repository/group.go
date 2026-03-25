package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/model"
)

func NewGroupRepository(db *sqlx.DB) GroupRepository { return &groupRepository{db} }

type GroupRepository interface {
	GetByName(model.GroupName) (*model.Group, error)
	All() ([]model.Group, error)
	UpdateGroups([]model.Group) error

	ValidateNameCase(model.GroupName) (model.GroupName, error)
	ValidateName(model.GroupName) (model.GroupName, error)

	InsertOrUpdateDepartment(*model.Department) error
	Departments() ([]model.Department, error)
	DepartmentIDs() ([]model.DepartmentID, error)

	GetTeacherByID(teacherID string) (*model.Teacher, error)
	Teachers() ([]model.Teacher, error)
	TeachersByChatID(chatID int) ([]model.Teacher, error)
	UpdateTeachers([]model.Teacher) error
}

type groupRepository struct{ db *sqlx.DB }

func (r *groupRepository) GetByName(name model.GroupName) (*model.Group, error) {
	var group model.Group
	err := r.db.Get(&group, "SELECT * FROM groups WHERE group_name = ?", name)
	return &group, err
}
func (r *groupRepository) All() ([]model.Group, error) {
	var groups []model.Group
	err := r.db.Select(&groups, "SELECT * FROM groups")
	return groups, err
}
func (r *groupRepository) UpdateGroups(groups []model.Group) error {
	log.Trace().Msg("Updating groups")

	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	for _, group := range groups {
		var _g model.Group
		if err := tx.Get(&_g, `SELECT * FROM groups WHERE group_id = ?`, group.GroupID); err != nil {
			// Insert new.
			_, err = tx.NamedExec(
				`INSERT INTO groups (group_id, department_id, group_name, department_name, year)
				VALUES (:group_id, :department_id, :group_name, :department_name, :year)`, group)
			if err != nil {
				tx.Rollback()
				return err
			}
		}

		// Update existing.
		group.UpdatedAt = time.Now()
		_, err = tx.NamedExec(
			`UPDATE groups
			SET department_id = :department_id, updated_at = :updated_at
			WHERE department_name = :department_name AND group_id = :group_id`,
			group,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (r *groupRepository) ValidateNameCase(name model.GroupName) (model.GroupName, error) {
	nameLower := strings.ToLower(string(name))

	groups, err := r.All()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to get groups from DB")
		return name, err
	}

	for _, group := range groups {
		groupNameLower := strings.ToLower(string(group.GroupName))
		if groupNameLower == nameLower {
			return group.GroupName, nil
		}
	}
	return name, errors.New("group name not found")
}
func (r *groupRepository) ValidateName(name model.GroupName) (model.GroupName, error) {
	validatedFormat, err := name.ValidateFormat()
	if err != nil {
		return name, err
	}
	return r.ValidateNameCase(validatedFormat)
}

func (r *groupRepository) InsertOrUpdateDepartment(department *model.Department) error {
	log.Trace().Any("department", department).Msg("Inserting...")
	_, err := r.db.NamedExec(`
		INSERT INTO departments (name, url)
		VALUES (:name, :url)
		ON CONFLICT (name) DO UPDATE SET url = :url, updated_at = CURRENT_TIMESTAMP
	`, department)
	return err
}
func (r *groupRepository) Departments() ([]model.Department, error) {
	var departments []model.Department
	err := r.db.Select(&departments, `SELECT id, name, url, created_at, updated_at FROM departments`)
	return departments, err
}
func (r *groupRepository) DepartmentIDs() ([]model.DepartmentID, error) {
	var ids []model.DepartmentID
	err := r.db.Select(&ids, `SELECT DISTINCT department_id FROM groups`)
	return ids, err
}

func (r *groupRepository) GetTeacherByID(teacherID string) (*model.Teacher, error) {
	var t model.Teacher
	err := r.db.Get(&t, `SELECT * FROM teachers WHERE teacher_id = ?`, teacherID)
	return &t, err
}
func (r *groupRepository) Teachers() ([]model.Teacher, error) {
	var teachers []model.Teacher
	err := r.db.Select(&teachers, "SELECT * FROM teachers")
	return teachers, err
}
func (r *groupRepository) TeachersByChatID(chatID int) ([]model.Teacher, error) {
	var teachers []model.Teacher
	err := r.db.Select(&teachers, `
			SELECT t.id, t.teacher_id, t.name, t.created_at, t.updated_at
			FROM recent_teachers rt JOIN teachers t ON rt.teacher_id = t.id
			WHERE rt.chat_id = ?
		`,
		chatID,
	)
	return teachers, err
}
func (r *groupRepository) UpdateTeachers(teachers []model.Teacher) error {
	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	for _, t := range teachers {
		var _t model.Teacher
		if err := tx.Get(&_t, `SELECT * FROM teachers WHERE teacher_id = ?`, t.TeacherID); err != nil {
			// Insert new
			_, err = tx.NamedExec(`INSERT INTO teachers (teacher_id, name) VALUES (:teacher_id, :name)`, t)
			if err != nil {
				tx.Rollback()
				return err
			}
		}

		// Update existing
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

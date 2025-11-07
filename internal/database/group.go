package database

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type Group struct {
	ID             int64     `db:"id" json:"id"`
	GroupID        string    `db:"group_id" json:"group_id"`
	DepartmentID   string    `db:"department_id" json:"department_id"`
	GroupName      string    `db:"group_name" json:"group_name"`
	DepartmentName string    `db:"department_name" json:"department_name"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

func (r *Repository) GetGroups() ([]Group, error) {
	var groups []Group
	err := r.db.Select(&groups, "SELECT * FROM groups")
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (r *Repository) GetGroupByName(name string) (*Group, error) {
	var group Group
	err := r.db.Get(&group, "SELECT * FROM groups WHERE group_name = ?", name)
	return &group, err
}

func (r *Repository) GetDepartmentIDs() (departmentIDs []string, err error) {
	err = r.db.Select(&departmentIDs, "SELECT DISTINCT department_id FROM groups")
	return
}

func (r *Repository) ValidateGroupNameCase(name string) (string, error) {
	nameLower := strings.ToLower(name)

	groups, err := r.GetGroups()
	if err != nil {
		return name, err
	}

	for _, group := range groups {
		if strings.ToLower(group.GroupName) == nameLower {
			return name, nil
		}
	}
	return name, errors.New("group name not found")
}

func (r *Repository) UpdateGroups(groups []Group) error {
	log.Trace().Msg("Updating groups")

	tx, err := r.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	for _, group := range groups {
		var _g Group
		if err := tx.Get(&_g, `SELECT * FROM groups WHERE group_id = ?`, group.GroupID); err != nil {
			// Insert new.
			_, err = tx.NamedExec(
				`INSERT INTO groups (group_id, department_id, group_name, department_name)
				VALUES (:group_id, :department_id, :group_name, :department_name)`, group)
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

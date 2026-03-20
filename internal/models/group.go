package models

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type GroupID string

func (i GroupID) String() string { return string(i) }

type DepartmentID string

func (i DepartmentID) String() string { return string(i) }

// GroupName is a string representing name of a student group.
type GroupName string

func (n GroupName) String() string { return string(n) }

const GroupRegexp = `^([\w\p{Cyrillic}]{3,5})[- ]*(\d{2})[- ]*\(?(9|11)\)?[- ]*(\d)$`

var GroupRE = regexp.MustCompile(GroupRegexp)

var ErrInvalidGroupNameFormat = errors.New("string does not match the group name format")

// ValidateFormat determines whether the given string can be formatted into a valid group name,
// and if it can, returns valid group name, else returns an error.
// It uses regexp provided by the constant [GroupRegexp].
//
// Important: the function doesn't validate case, i.e. if string "иСпТ-22-(9)-2" is given, the result is the same.
func (n GroupName) ValidateFormat() (GroupName, error) {
	if !GroupRE.MatchString(string(n)) {
		return n, fmt.Errorf("%w: '%s'", ErrInvalidGroupNameFormat, n)
	}
	subs := GroupRE.FindStringSubmatch(string(n))
	return GroupName(fmt.Sprintf("%s-%s-(%s)-%s", subs[1], subs[2], subs[3], subs[4])), nil
}

func (group GroupName) Parse() (name string, year int, base int, n int, err error) {
	if !GroupRE.MatchString(string(group)) {
		return "", 0, 0, 0, fmt.Errorf("%w: '%s'", ErrInvalidGroupNameFormat, group)
	}
	subs := GroupRE.FindStringSubmatch(string(group))
	year, _ = strconv.Atoi(subs[2])
	base, _ = strconv.Atoi(subs[3])
	n, _ = strconv.Atoi(subs[4])
	return subs[1], year, base, n, nil
}

type Year int

type Group struct {
	ID             int64          `db:"id"              json:"id"`
	GroupID        GroupID        `db:"group_id"        json:"group_id"`
	DepartmentID   DepartmentID   `db:"department_id"   json:"department_id"`
	GroupName      GroupName      `db:"group_name"      json:"group_name"`
	DepartmentName DepartmentName `db:"department_name" json:"department_name"`
	Year           Year           `db:"year"            json:"year"`
	CreatedAt      time.Time      `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"      json:"updated_at"`
}

func GetGroups(db *sqlx.DB) ([]Group, error) {
	var groups []Group
	if err := db.Select(&groups, "SELECT * FROM groups"); err != nil {
		return nil, err
	}
	return groups, nil
}

func GetGroupByName(db *sqlx.DB, name GroupName) (*Group, error) {
	var group Group
	err := db.Get(&group, "SELECT * FROM groups WHERE group_name = ?", name)
	return &group, err
}

func GetDepartmentIDs(db *sqlx.DB) (departmentIDs []DepartmentID, err error) {
	err = db.Select(&departmentIDs, "SELECT DISTINCT department_id FROM groups")
	return
}

// ValidateGroupNameCase validates group name case. Argument value must has valid format.
func ValidateGroupNameCase(db *sqlx.DB, name GroupName) (GroupName, error) {
	nameLower := strings.ToLower(string(name))

	groups, err := GetGroups(db)
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

// ValidateGroupName validate group name format and case.
func ValidateGroupName(db *sqlx.DB, name GroupName) (GroupName, error) {
	validatedFormat, err := name.ValidateFormat()
	if err != nil {
		return name, err
	}
	return ValidateGroupNameCase(db, validatedFormat)
}

func UpdateGroups(db *sqlx.DB, groups []Group) error {
	log.Trace().Msg("Updating groups")

	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	for _, group := range groups {
		var _g Group
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

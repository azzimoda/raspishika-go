package models

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type MigrationName string

type Migration struct {
	ID        int64         `db:"id"`
	Name      MigrationName `db:"name"`
	CreatedAt time.Time     `db:"created_at"`
}

func EnsureMigrationsTable(db *sqlx.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS migrations (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to ensure migration table: %w", err)
	}
	return nil
}

// checkMigration checks if a migration exists in the database. If it does not exist, it returns an error.
func CheckMigration(db *sqlx.DB, name MigrationName) error {
	var exists bool
	if err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM migrations WHERE name = ?)", name); err != nil {
		return fmt.Errorf("failed to check migration %s: %w", name, err)
	}
	if exists {
		return nil
	}
	return fmt.Errorf("migration %s does not exist", name)
}

func ApplyMigration(db *sqlx.DB, name MigrationName, sql string) error {
	if _, err := db.Exec(sql); err != nil {
		return fmt.Errorf("failed to apply migration %s: %w", name, err)
	}

	if _, err := db.Exec("INSERT INTO migrations (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("failed to record migration %s: %w", name, err)
	}

	return nil
}

func GetMigrations(db *sqlx.DB) ([]Migration, error) {
	var migrations []Migration
	if err := db.Select(&migrations, "SELECT * From migrations"); err != nil {
		return nil, err
	}
	return migrations, nil
}

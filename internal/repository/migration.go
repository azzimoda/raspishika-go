package repository

import (
	"fmt"
	"time"
)

type Migration struct {
	ID        int64     `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

func (r *Repository) ensureMigrationsTable() error {
	_, err := r.db.Exec(`CREATE TABLE IF NOT EXISTS migrations (
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
func (r *Repository) checkMigration(name string) error {
	var exists bool
	if err := r.db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM migrations WHERE name = ?)", name); err != nil {
		return fmt.Errorf("failed to check migration %s: %w", name, err)
	}
	if exists {
		return nil
	}
	return fmt.Errorf("migration %s does not exist", name)
}

func (r *Repository) applyMigration(name, sql string) error {
	if _, err := r.db.Exec(sql); err != nil {
		return fmt.Errorf("failed to apply migration %s: %w", name, err)
	}

	if _, err := r.db.Exec("INSERT INTO migrations (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("failed to record migration %s: %w", name, err)
	}

	return nil
}

func (r *Repository) getMigrations() ([]Migration, error) {
	var migrations []Migration
	if err := r.db.Select(&migrations, "SELECT * From migrations"); err != nil {
		return nil, err
	}
	return migrations, nil
}

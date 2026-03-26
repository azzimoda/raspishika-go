package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/config"
)

type migrationFile struct {
	name string
	sql  string
}

// checkMigration checks if a migration exists in the database. If it does not exist, it returns an error.
func checkMigration(db *sqlx.DB, name string) error {
	var exists bool
	if err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM migrations WHERE name = ?)", name); err != nil {
		return fmt.Errorf("failed to check migration %s: %w", name, err)
	}
	if exists {
		return nil
	}
	return fmt.Errorf("migration %s does not exist", name)
}

func applyMigration(db *sqlx.DB, name string, sql string) error {
	if _, err := db.Exec(sql); err != nil {
		return fmt.Errorf("failed to apply migration %s: %w", name, err)
	}

	if _, err := db.Exec("INSERT INTO migrations (name) VALUES (?)", name); err != nil {
		return fmt.Errorf("failed to record migration %s: %w", name, err)
	}

	return nil
}

func applyMigrations(db *sqlx.DB) error {
	log.Trace().Msg("Applying migrations...")
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to ensure migrations table: %w", err)
	}

	files, err := migrationFiles()
	if err != nil {
		return fmt.Errorf("failed to load migration files: %w", err)
	}
	log.Trace().Int("count", len(files)).Msg("Loaded migration files")

	count := 0
	for _, file := range files {
		name := file.name
		if err := checkMigration(db, name); err != nil {
			if err := applyMigration(db, name, file.sql); err != nil {
				log.Error().Err(err).Any("name", name).Msg("Failed to apply migration")
				return fmt.Errorf("failed to apply migration %s: %w", name, err)
			}
			log.Debug().Any("name", name).Msg("Applied migration")
			count++
		} else {
			log.Trace().Any("name", name).Msg("Skipped migration")
		}
	}
	log.Debug().Int("count", count).Msg("Migrations applied")

	return nil
}

func migrationFiles() ([]migrationFile, error) {
	dir := viper.GetString(config.KeyDatabaseMigrations)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Warn().Str("dir", dir).Msg("Migrations directory doesn't exist")
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create migrations directory %s: %w", dir, err)
		}
		log.Debug().Msg("Created migrations directory")
		return make([]migrationFile, 0), nil // That means the directory is empty
	}

	log.Trace().Str("migrationsDir", dir).Send()
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, fmt.Errorf("failed to load migration files: %w", err)
	}
	log.Trace().Int("count", len(files)).Msg("Found migration files")

	var migrations []migrationFile
	for _, file := range files {
		name := filepath.Base(file)
		sql, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %w", name, err)
		}
		migrations = append(migrations, migrationFile{name: name, sql: string(sql)})
	}
	return migrations, nil
}

func ensureMigrationsTable(db *sqlx.DB) error {
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

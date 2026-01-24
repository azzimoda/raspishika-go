package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

func New() (*Repository, error) {
	file := viper.GetString("database.file")
	log.Debug().Str("file", file).Msg("Creating database repository")
	db, err := sqlx.Open("sqlite3", file)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	r := &Repository{db: db}
	if err := r.applyMigrations(); err != nil {
		return nil, err
	}
	return r, nil
}

type Repository struct {
	db *sqlx.DB
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) applyMigrations() error {
	log.Trace().Msg("Applying migrations...")
	if err := r.ensureMigrationsTable(); err != nil {
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
		sql := file.sql
		if err := r.checkMigration(name); err != nil {
			if err := r.applyMigration(name, sql); err != nil {
				log.Error().Err(err).Str("name", name).Msg("Failed to apply migration")
				return fmt.Errorf("failed to apply migration %s: %w", name, err)
			}
			log.Debug().Str("name", name).Msg("Applied migration")
			count++
		} else {
			log.Trace().Str("name", name).Msg("Skipped migration")
		}
	}
	log.Debug().Int("count", count).Msg("Migrations applied")

	return nil
}

func migrationFiles() ([]migrationFile, error) {
	migrationsDir := viper.GetString("database.migrations")
	log.Trace().Str("migrationsDir", migrationsDir).Send()
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
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

type migrationFile struct {
	name string
	sql  string
}

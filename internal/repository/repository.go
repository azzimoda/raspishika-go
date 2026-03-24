package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/model"
)

func New(db *sqlx.DB) (*Repository, error) {
	file := viper.GetString(config.KeyDatabaseFile)
	if _, err := os.Stat(file); err != nil {
		log.Warn().Str("file", file).Msg("Database file or its directore doesn't exist")
		if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", file, err)
		}
		log.Debug().Msg("Database directory created")
	}

	log.Debug().Str("file", file).Msg("Creating database repository")
	db, err := sqlx.Open("sqlite3", file)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	r := &Repository{DB: db}
	if err := r.applyMigrations(); err != nil {
		return nil, err
	}
	return r, nil
}

type Repository struct{ *sqlx.DB }

func (r *Repository) Close() error { return r.DB.Close() }

func (r *Repository) applyMigrations() error {
	log.Trace().Msg("Applying migrations...")
	if err := model.EnsureMigrationsTable(r.DB); err != nil {
		return fmt.Errorf("failed to ensure migrations table: %w", err)
	}

	files, err := migrationFiles()
	if err != nil {
		return fmt.Errorf("failed to load migration files: %w", err)
	}
	log.Trace().Int("count", len(files)).Msg("Loaded migration files")

	count := 0
	for _, file := range files {
		name := model.MigrationName(file.name)
		sql := file.sql
		if err := model.CheckMigration(r.DB, name); err != nil {
			if err := model.ApplyMigration(r.DB, name, sql); err != nil {
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

type migrationFile struct {
	name string
	sql  string
}

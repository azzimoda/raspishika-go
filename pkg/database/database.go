package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/config"
)

func New() (*sqlx.DB, error) {
	file := viper.GetString(config.KeyDatabaseFile)
	log.Debug().Str("file", file).Msg("SQLite database file")

	if _, err := os.Stat(file); err != nil {
		log.Warn().Str("file", file).Msg("Database file doesn't exist")
		dir := filepath.Dir(file)
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", file, err)
		}
		log.Debug().Str("dir", dir).Msg("Database directory created")
	}

	db, err := sqlx.Open("sqlite3", file)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := applyMigrations(db); err != nil {
		return nil, err
	}

	return db, nil
}

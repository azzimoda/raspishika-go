package database

import (
	"github.com/azzimoda/raspishika-go/internal/config"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

type Repository struct {
	db *sqlx.DB
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func New(cfg *config.Config) (*Repository, error) {
	db, err := sqlx.Open("sqlite3", cfg.Database.File)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return &Repository{db: db}, nil
}

func createTables(db *sqlx.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS chats(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL UNIQUE,
			username TEXT,
			state TEXT DEFAULT 'default',
			department TEXT,
			"group" TEXT,
			daily_sending_time TEXT NOT NULL DEFAULT '',
			pair_sending BOOLEAN NOT NULL DEFAULT 0,
			access INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS groups(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id TEXT NOT NULL UNIQUE,
			department_id TEXT NOT NULL,
			group_name TEXT NOT NULL,
			department_name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS teachers(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			teacher_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS recent_teachers(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER NOT NULL REFERENCES chat(id) ON DELETE CASCADE ON UPDATE CASCADE,
			teacher_id INTEGER NOT NULL REFERENCES teachers(id) ON DELETE CASCADE ON UPDATE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS update_logs(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER REFERENCES chat(id) ON UPDATE CASCADE,
			kind TEXT NOT NULL,
			message_id INTEGER,
			data TEXT,
			handling_time INTEGER NOT NULL DEFAULT 0, -- milliseconds
			error TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Error().Str("query", query).Msg("Failed to execute query")
			return err
		}
	}
	return nil
}

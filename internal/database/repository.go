package database

import (
	"github.com/azzimoda/raspishika-go/internal/config"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
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
			department TEXT,
			"group" TEXT,
			daily_sending_time TEXT NOT NULL DEFAULT '',
			pair_sending BOOLEAN NOT NULL DEFAULT 0,
			access INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}
	return nil
}

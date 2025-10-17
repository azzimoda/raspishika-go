package database

import (
	"database/sql"

	"github.com/azzimoda/raspishika-go/internal/config"

	_ "github.com/mattn/go-sqlite3"
)

type Repository struct {
	db *sql.DB
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func New(cfg *config.Config) (*Repository, error) {
	db, err := sql.Open("sqlite3", cfg.Database.File)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Repository{db: db}, nil
}

// Package store отвечает за работу с базой данных: подключение, схему и запросы.
package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // чистый Go-драйвер SQLite, без CGO
)

type Store struct {
	db *sql.DB
}

// Open открывает БД, создаёт схему и заполняет тестовыми данными, если БД пустая.
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := s.seed(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY,
	login         TEXT    NOT NULL UNIQUE,
	password_hash TEXT    NOT NULL,
	is_active     INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS channels (
	id          INTEGER PRIMARY KEY,
	number      INTEGER NOT NULL UNIQUE,
	name        TEXT    NOT NULL,
	stream_path TEXT    NOT NULL,
	is_active   INTEGER NOT NULL DEFAULT 1
);
`

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

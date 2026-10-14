package store

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type seedUser struct {
	login, password string
	active          bool
}

type seedChannel struct {
	number     int
	name, path string
	active     bool
}

// Тестовые данные. Пароли попадают в БД только в виде bcrypt-хэша.
var (
	seedUsers = []seedUser{
		{"alice", "alice-pass", true},
		{"bob", "bob-pass", true},
		{"disabled", "disabled-pass", false}, // для проверки 401 у выключенного пользователя
	}
	seedChannels = []seedChannel{
		{1, "Первый", "/live/ch1/index.m3u8", true},
		{2, "Россия 1", "/live/ch2/index.m3u8", true},
		{3, "НТВ", "/live/ch3/index.m3u8", true},
		{4, "СТС", "/live/ch4/index.m3u8", true},
		{5, "Культура", "/live/ch5/index.m3u8", true},
		{6, "Архивный", "/live/ch6/index.m3u8", false}, // для проверки 404 у выключенного канала
	}
)

// seed заполняет пустые таблицы. Повторный запуск ничего не меняет.
func (s *Store) seed(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		for _, u := range seedUsers {
			hash, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO users (login, password_hash, is_active) VALUES (?, ?, ?)`,
				u.login, string(hash), u.active); err != nil {
				return err
			}
		}
	}

	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM channels`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		for _, c := range seedChannels {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO channels (number, name, stream_path, is_active) VALUES (?, ?, ?, ?)`,
				c.number, c.name, c.path, c.active); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

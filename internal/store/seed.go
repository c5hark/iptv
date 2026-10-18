package store

import (
	"context"
	"fmt"
	"time"

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

type seedPackage struct {
	name     string
	channels []int
}

type seedSubscription struct {
	login, pkg string
	expiresIn  *time.Duration
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
		{3, "БСТ", "/live/ch3/index.m3u8", true},
		{4, "СТС", "/live/ch4/index.m3u8", true},
		{5, "Уфанет", "/live/ch5/index.m3u8", true},
		{6, "Архивный", "/live/ch6/index.m3u8", false}, // для проверки 404 у выключенного канала
	}
	seedPackages = []seedPackage{
		{"Базовый", []int{1, 2, 3}},
		{"Расширенный", []int{4, 5, 6}},
	}
)

func ptr[T any](v T) *T { return &v }

var seedSubscriptions = []seedSubscription{
	{"alice", "Базовый", nil},
	{"alice", "Расширенный", ptr(365 * 24 * time.Hour)},
	{"bob", "Базовый", nil},
	{"bob", "Расширенный", ptr(-24 * time.Hour)},
}

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

	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM packages`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		for _, p := range seedPackages {
			res, err := tx.ExecContext(ctx, `INSERT INTO packages (name) VALUES (?)`, p.name)
			if err != nil {
				return err
			}
			pkgID, err := res.LastInsertId()
			if err != nil {
				return err
			}
			for _, number := range p.channels {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO package_channels (package_id, channel_id)
							SELECT ?, id FROM channels WHERE number = ?`,
					pkgID, number); err != nil {
					return err
				}
			}
		}

		now := time.Now()
		for _, sub := range seedSubscriptions {
			var expiresAt any
			if sub.expiresIn != nil {
				expiresAt = now.Add(*sub.expiresIn).Unix()
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO subscriptions (user_id, package_id, is_active, expires_at)
						SELECT u.id, p.id, 1, ? FROM users u, packages p
						WHERE u.login = ? AND p.name = ?`,
				expiresAt, sub.login, sub.pkg); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

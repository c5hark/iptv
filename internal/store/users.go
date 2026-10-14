package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound возвращается, когда запись не найдена.
var ErrNotFound = errors.New("not found")

type User struct {
	ID           int64
	Login        string
	PasswordHash string
	IsActive     bool
}

// UserByLogin ищет пользователя по логину. Если его нет, возвращает ErrNotFound.
func (s *Store) UserByLogin(ctx context.Context, login string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, login, password_hash, is_active FROM users WHERE login = ?`, login,
	).Scan(&u.ID, &u.Login, &u.PasswordHash, &u.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	return u, nil
}

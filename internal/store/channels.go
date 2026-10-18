package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Channel struct {
	ID         int64
	Number     int
	Name       string
	StreamPath string
	isActive   bool
}

// ListActiveChannels возвращает включенные каналы отсортированные по номеру
func (s *Store) ListActiveChannels(ctx context.Context) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, number, name, stream_path, is_active
		FROM channels WHERE is_active = 1 ORDER BY number`)
	if err != nil {
		return nil, fmt.Errorf("query channels: %w", err)
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.Number, &c.Name, &c.StreamPath, &c.isActive); err != nil {
			return nil, fmt.Errorf("scan channels: %w", err)
		}
		channels = append(channels, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}
	return channels, nil
}

// ActiveChannelsByID ищет включенный канал. Если его нет или он выключен, возвращает ErrNotFound
func (s *Store) ActiveChannelByID(ctx context.Context, id int64) (Channel, error) {
	var c Channel
	err := s.db.QueryRowContext(ctx,
		`SELECT id, number, name, stream_path, is_active
		FROM channels WHERE id = ? AND is_active = 1`, id,
	).Scan(&c.ID, &c.Number, &c.Name, &c.StreamPath, &c.isActive)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, ErrNotFound
	}
	if err != nil {
		return Channel{}, fmt.Errorf("query channels: %w", err)
	}
	return c, nil
}

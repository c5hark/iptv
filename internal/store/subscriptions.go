package store

import (
	"context"
	"fmt"
	"time"
)

// activeSubscription — условие «подписка сейчас действует»: включена вручную и не истекла.
// NULL в expires_at — бессрочная подписка. Ровно в момент expires_at подписка уже не действует.
// Параметр ? — текущее время в unix-секундах. Время передаём снаружи, а не берём из SQL. Так лучше будет для тестов
const activeSubscription = `s.is_active = 1 AND (s.expires_at IS NULL OR s.expires_at > ?)`

// ListAccessibleChannels возвращает включенные каналы, доступные пользователю на момент now
// по порядку номеров. Канал в нескольких пакетах попадает в список один раз.
func (s *Store) ListAccessibleChannels(ctx context.Context, userID int64, now time.Time) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT c.id, c.number, c.name, c.stream_path, c.is_active
		FROM channels c
		JOIN package_channels pc ON c.id = pc.channel_id
		JOIN subscriptions s     ON s.package_id = pc.package_id
		WHERE s.user_id = ? AND `+activeSubscription+` AND c.is_active = 1
		ORDER BY c.number`,
		userID, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("query accessible channels: %w", err)
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.Number, &c.Name, &c.StreamPath, &c.isActive); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}
	return channels, nil
}

// HasChannelAccess отвечает, есть ли у пользователя на момент now доступ к каналу.
// Правило как в ListAccessibleChannels.
func (s *Store) HasChannelAccess(ctx context.Context, userID int64, channelID int64, now time.Time) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM package_channels pc
			JOIN subscriptions s ON s.package_id = pc.package_id
			JOIN channels c ON c.id = pc.channel_id
			WHERE s.user_id = ? AND pc.channel_id = ? AND c.is_active = 1 AND `+activeSubscription+`
		)`,
		userID, channelID, now.Unix()).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("query channel access: %w", err)
	}
	return ok, nil
}

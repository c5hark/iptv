package store

import (
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // выполнится раньше удаления TempDir (важно на Windows)
	return st
}

func userID(t *testing.T, st *Store, login string) int64 {
	t.Helper()
	u, err := st.UserByLogin(t.Context(), login)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// accessibleNumbers возвращает номера каналов, доступных пользователю на момент now.
func accessibleNumbers(t *testing.T, st *Store, login string, now time.Time) []int {
	t.Helper()
	channels, err := st.ListAccessibleChannels(t.Context(), userID(t, st, login), now)
	if err != nil {
		t.Fatal(err)
	}
	numbers := []int{}
	for _, c := range channels {
		numbers = append(numbers, c.Number)
	}
	return numbers
}

func TestListAccessibleChannels(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()

	tests := []struct {
		name  string
		login string
		at    time.Time
		want  []int
	}{
		// alice: бессрочный «Базовый» (1–3) и «Расширенный» на год (4–6, но канал 6 выключен).
		{"perpetual and limited subscriptions", "alice", now, []int{1, 2, 3, 4, 5}},
		// Через два года подписка с ограничением истекла сама, бессрочная осталась.
		{"limited subscription expires on its own", "alice", now.AddDate(2, 0, 0), []int{1, 2, 3}},
		// bob: «Расширенный» истёк вчера.
		{"already expired subscription", "bob", now, []int{1, 2, 3}},
		{"user without subscriptions", "disabled", now, []int{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := accessibleNumbers(t, st, tc.login, tc.at)
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestManualDeactivationOfPerpetualSubscription(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()

	if got := accessibleNumbers(t, st, "bob", now); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("before: got %v", got)
	}

	// Администратор выключает бессрочную подписку вручную.
	_, err := st.db.ExecContext(t.Context(),
		`UPDATE subscriptions SET is_active = 0
		 WHERE user_id = ? AND package_id = (SELECT id FROM packages WHERE name = 'Базовый')`,
		userID(t, st, "bob"))
	if err != nil {
		t.Fatal(err)
	}

	if got := accessibleNumbers(t, st, "bob", now); len(got) != 0 {
		t.Errorf("after deactivation: got %v, want none", got)
	}
}

func TestExpiryBoundary(t *testing.T) {
	st := newTestStore(t)
	expiry := time.Now().Add(time.Hour).Truncate(time.Second)

	_, err := st.db.ExecContext(t.Context(),
		`UPDATE subscriptions SET expires_at = ?
		 WHERE user_id = ? AND package_id = (SELECT id FROM packages WHERE name = 'Расширенный')`,
		expiry.Unix(), userID(t, st, "alice"))
	if err != nil {
		t.Fatal(err)
	}

	before := accessibleNumbers(t, st, "alice", expiry.Add(-time.Second))
	if !slices.Contains(before, 4) {
		t.Errorf("one second before expiry: got %v, want channel 4", before)
	}
	atExpiry := accessibleNumbers(t, st, "alice", expiry)
	if slices.Contains(atExpiry, 4) {
		t.Errorf("exactly at expiry: got %v, channel 4 must be gone", atExpiry)
	}
}

// Список и точечная проверка доступа должны давать один и тот же ответ.
func TestHasChannelAccessMatchesList(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()

	for _, login := range []string{"alice", "bob", "disabled"} {
		accessible := accessibleNumbers(t, st, login, now)
		for number := 1; number <= 6; number++ {
			var channelID int64
			err := st.db.QueryRowContext(t.Context(),
				`SELECT id FROM channels WHERE number = ?`, number).Scan(&channelID)
			if err != nil {
				t.Fatal(err)
			}
			got, err := st.HasChannelAccess(t.Context(), userID(t, st, login), channelID, now)
			if err != nil {
				t.Fatal(err)
			}
			want := slices.Contains(accessible, number)
			if got != want {
				t.Errorf("%s, channel %d: HasChannelAccess = %v, list says %v", login, number, got, want)
			}
		}
	}
}

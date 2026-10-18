package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"iptv/internal/store"
)

// newTestRouterAt — как newTestRouter, но с заданным «текущим временем».
func newTestRouterAt(t *testing.T, now func() time.Time) http.Handler {
	t.Helper()
	st, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newRouter(st, "http://stream.test", logger, now)
}

// inTwoYears — часы, показывающие момент через два года: подписка alice на год к тому времени истекла.
func inTwoYears() time.Time { return time.Now().AddDate(2, 0, 0) }

func listedNumbers(t *testing.T, h http.Handler, login, password string) []int {
	t.Helper()
	rec := get(t, h, "/api/v1/channels", login, password)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body channelsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	numbers := []int{}
	for _, c := range body.Channels {
		numbers = append(numbers, c.Number)
	}
	return numbers
}

func TestListShowsOnlySubscribedChannels(t *testing.T) {
	h := newTestRouterAt(t, time.Now)

	// alice: «Базовый» + «Расширенный» (канал 6 выключен). bob: только «Базовый»,
	// его «Расширенный» истёк вчера.
	if got := listedNumbers(t, h, "alice", "alice-pass"); !slices.Equal(got, []int{1, 2, 3, 4, 5}) {
		t.Errorf("alice sees %v", got)
	}
	if got := listedNumbers(t, h, "bob", "bob-pass"); !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("bob sees %v", got)
	}
}

func TestListAfterLimitedSubscriptionExpires(t *testing.T) {
	h := newTestRouterAt(t, inTwoYears)

	if got := listedNumbers(t, h, "alice", "alice-pass"); !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("alice sees %v, want only the perpetual package", got)
	}
}

func TestPlaybackRequiresSubscription(t *testing.T) {
	tests := []struct {
		name       string
		now        func() time.Time
		login, pwd string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"subscribed channel", time.Now, "bob", "bob-pass", "/api/v1/channels/1/playback", 200, ""},
		{"expired subscription", time.Now, "bob", "bob-pass", "/api/v1/channels/4/playback", 403, "forbidden"},
		{"active limited subscription", time.Now, "alice", "alice-pass", "/api/v1/channels/4/playback", 200, ""},
		{"limited subscription ran out", inTwoYears, "alice", "alice-pass", "/api/v1/channels/4/playback", 403, "forbidden"},
		// Выключенный канал — 404, даже если он входит в оплаченный пакет.
		{"inactive channel in paid package", time.Now, "alice", "alice-pass", "/api/v1/channels/6/playback", 404, "not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestRouterAt(t, tc.now)
			rec := get(t, h, tc.path, tc.login, tc.pwd)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode == "" {
				return
			}
			var body struct {
				Error struct{ Code, Message string } `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body is not JSON: %v", err)
			}
			if body.Error.Code != tc.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tc.wantCode)
			}
		})
	}
}

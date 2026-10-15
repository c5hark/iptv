package api

import (
	"encoding/json"
	"io"
	"iptv/internal/store"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// newTestRouter поднимает роутер на настоящей SQLite во временной папке:
// тест проверяет поведение API целиком, включая SQL и seed-данные.
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // выполнится раньше удаления TempDir (важно на Windows)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(st, "http://stream.test/", logger)
}

func get(t *testing.T, h http.Handler, path, login, password string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if login != "" {
		req.SetBasicAuth(login, password)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestChannelsRequireAuth(t *testing.T) {
	h := newTestRouter(t)
	for _, path := range []string{"/api/v1/channels", "/api/v1/channels/1/playback"} {
		rec := get(t, h, path, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, rec.Code)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%s: missing WWW-Authenticate", path)
		}
	}
}

func TestListChannels(t *testing.T) {
	h := newTestRouter(t)
	rec := get(t, h, "/api/v1/channels", "alice", "alice-pass")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var body channelsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// В seed 6 каналов, один выключен: в списке должно быть 5, по порядку номеров.
	if len(body.Channels) != 5 {
		t.Fatalf("got %d channels, want 5", len(body.Channels))
	}
	for i, c := range body.Channels {
		if c.Number != i+1 {
			t.Errorf("channel[%d].number = %d, want %d", i, c.Number, i+1)
		}
		if c.Name == "Архивный" {
			t.Error("inactive channel returned in list")
		}
	}
}

func TestPlayback(t *testing.T) {
	h := newTestRouter(t)

	t.Run("ok", func(t *testing.T) {
		rec := get(t, h, "/api/v1/channels/1/playback", "alice", "alice-pass")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body playbackResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if body.ChannelID != 1 || body.URL != "http://stream.test/live/ch1/index.m3u8" {
			t.Errorf("unexpected body: %+v", body)
		}

		var raw map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		if _, ok := raw["channel_id"]; !ok {
			t.Errorf("response has no channel_id key: %s", rec.Body.String())
		}
	})

	errorCases := []struct {
		name     string
		path     string
		wantCode int
		wantErr  string
	}{
		{"inactive channel", "/api/v1/channels/6/playback", 404, "not_found"},
		{"missing channel", "/api/v1/channels/999/playback", 404, "not_found"},
		{"not a number", "/api/v1/channels/abc/playback", 400, "invalid_id"},
		{"zero", "/api/v1/channels/0/playback", 400, "invalid_id"},
		{"negative", "/api/v1/channels/-1/playback", 400, "invalid_id"},
		{"overflow", "/api/v1/channels/99999999999999999999/playback", 400, "invalid_id"},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, h, tc.path, "alice", "alice-pass")
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			var body struct {
				Error struct{ Code, Message string } `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body is not JSON: %v", err)
			}
			if body.Error.Code != tc.wantErr {
				t.Errorf("error code = %q, want %q", body.Error.Code, tc.wantErr)
			}
		})
	}
}

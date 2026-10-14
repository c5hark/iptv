package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"iptv/internal/store"
)

type fakeUsers struct {
	users map[string]store.User
	err   error
}

func (f fakeUsers) UserByLogin(_ context.Context, login string) (store.User, error) {
	if f.err != nil {
		return store.User{}, f.err
	}
	u, ok := f.users[login]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func hash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestBasic(t *testing.T) {
	users := fakeUsers{users: map[string]store.User{
		"alice":    {ID: 1, Login: "alice", PasswordHash: hash(t, "secret"), IsActive: true},
		"disabled": {ID: 2, Login: "disabled", PasswordHash: hash(t, "secret"), IsActive: false},
	}}

	tests := []struct {
		name       string
		users      fakeUsers
		login, pwd string
		noAuth     bool
		wantStatus int
		wantCode   string // код ошибки в JSON, если ожидается ошибка
	}{
		{name: "valid credentials", users: users, login: "alice", pwd: "secret", wantStatus: 200},
		{name: "no header", users: users, noAuth: true, wantStatus: 401, wantCode: "unauthorized"},
		{name: "wrong password", users: users, login: "alice", pwd: "nope", wantStatus: 401, wantCode: "unauthorized"},
		{name: "unknown user", users: users, login: "ghost", pwd: "secret", wantStatus: 401, wantCode: "unauthorized"},
		{name: "inactive user", users: users, login: "disabled", pwd: "secret", wantStatus: 401, wantCode: "unauthorized"},
		{name: "too long password", users: users, login: "alice", pwd: strings.Repeat("a", 1000), wantStatus: 401, wantCode: "unauthorized"},
		{name: "storage error", users: fakeUsers{err: errors.New("SELECT * FROM secret_table failed")}, login: "alice", pwd: "secret", wantStatus: 500, wantCode: "internal"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			var gotUser store.User
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotUser, _ = UserFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})
			h := Basic(tc.users, logger)(next)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if !tc.noAuth {
				req.SetBasicAuth(tc.login, tc.pwd)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.wantStatus == 200 {
				if gotUser.Login != "alice" {
					t.Errorf("user in context = %q, want alice", gotUser.Login)
				}
				return
			}

			if tc.wantStatus == 401 && rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("missing WWW-Authenticate header on 401")
			}
			var body struct {
				Error struct{ Code, Message string } `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %v", err)
			}
			if body.Error.Code != tc.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tc.wantCode)
			}
			if strings.Contains(rec.Body.String(), "SELECT") {
				t.Error("internal error details leaked to the client")
			}
		})
	}
}

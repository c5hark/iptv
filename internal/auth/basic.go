// Package auth реализует HTTP Basic Auth поверх хранилища пользователей.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"iptv/internal/httpx"
	"iptv/internal/store"
)

// Users — то, что нужно middleware от хранилища. Интерфейс объявлен здесь,
// у потребителя, чтобы в тестах подставлять простую заглушку.
type Users interface {
	UserByLogin(ctx context.Context, login string) (store.User, error)
}

// Ограничение на длину логина и пароля: не даём гонять bcrypt на огромных входных данных.
const maxCredentialLen = 256

// dummyHash нужен, чтобы для несуществующего логина всё равно выполнить bcrypt.
// Иначе по времени ответа можно понять, существует ли логин.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type ctxKey struct{}

// UserFromContext достаёт аутентифицированного пользователя, которого положил Basic.
func UserFromContext(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(ctxKey{}).(store.User)
	return u, ok
}

// Basic возвращает middleware: пропускает запрос, только если пользователь
// существует, активен и пароль верный. Иначе 401 с WWW-Authenticate.
func Basic(users Users, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			login, password, ok := r.BasicAuth()
			if !ok || len(login) > maxCredentialLen || len(password) > maxCredentialLen {
				unauthorized(w)
				return
			}

			user, err := users.UserByLogin(r.Context(), login)
			switch {
			case errors.Is(err, store.ErrNotFound):
				_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
				unauthorized(w)
				return
			case err != nil:
				// Подробности только в лог, клиенту — общее сообщение.
				logger.Error("auth: user lookup failed", "err", err)
				httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				return
			}

			// Хэш проверяем до проверки is_active, чтобы время ответа не зависело от неё.
			passwordOK := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil
			if !passwordOK || !user.IsActive {
				unauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="iptv", charset="UTF-8"`)
	httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
}

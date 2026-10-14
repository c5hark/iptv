// Package api собирает HTTP-маршруты сервиса.
package api

import (
	"log/slog"
	"net/http"

	"iptv/internal/auth"
	"iptv/internal/httpx"
)

// NewRouter собирает все маршруты. /healthz открыт всем, всё остальное — только после Basic Auth.
func NewRouter(users auth.Users, logger *slog.Logger) http.Handler {
	protected := http.NewServeMux()
	// TODO(шаг 4): здесь появятся GET /api/v1/channels и GET /api/v1/channels/{id}/playback.
	protected.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "resource not found")
	})

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	root.Handle("/", auth.Basic(users, logger)(protected))
	return root
}

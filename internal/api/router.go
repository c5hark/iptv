// Package api собирает HTTP-маршруты сервиса.
package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"iptv/internal/auth"
	"iptv/internal/httpx"
	"iptv/internal/store"
)

// NewRouter собирает все маршруты. /healthz открыт всем, всё остальное — только после Basic Auth.
func NewRouter(st *store.Store, streamBaseURL string, logger *slog.Logger) http.Handler {
	return newRouter(st, streamBaseURL, logger, time.Now)
}

// newRouter — то же, но с подменяемым источником времени (нужен тестам подписок).
func newRouter(st *store.Store, streamBaseURL string, logger *slog.Logger, now func() time.Time) http.Handler {
	channels := &channelsHandler{
		store:         st,
		streamBaseURL: strings.TrimRight(streamBaseURL, "/"),
		now:           now,
		logger:        logger,
	}

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/channels", channels.list)
	protected.HandleFunc("GET /api/v1/channels/{id}/playback", channels.playback)
	protected.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "resource not found")
	})

	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	root.Handle("/", auth.Basic(st, logger)(protected))
	return root
}

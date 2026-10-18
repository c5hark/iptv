package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"iptv/internal/auth"
	"iptv/internal/httpx"
	"iptv/internal/store"
)

type channelsHandler struct {
	store         *store.Store
	streamBaseURL string
	now           func() time.Time
	logger        *slog.Logger
}

type channelDTO struct {
	ID     int64  `json:"id"`
	Number int    `json:"number"`
	Name   string `json:"name"`
}

type channelsResponse struct {
	Channels []channelDTO `json:"channels"`
}

type playbackResponse struct {
	ChannelID int64  `json:"channel_id"`
	URL       string `json:"url"`
}

// list: GET /api/v1/channels — только каналы с действующей подпиской.
func (h *channelsHandler) list(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}

	channels, err := h.store.ListAccessibleChannels(r.Context(), user.ID, h.now())
	if err != nil {
		h.internalError(w, "list channels", err)
		return
	}

	resp := channelsResponse{Channels: make([]channelDTO, 0, len(channels))}
	for _, c := range channels {
		resp.Channels = append(resp.Channels, channelDTO{ID: c.ID, Number: c.Number, Name: c.Name})
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// playback: GET /api/v1/channels/{id}/playback
// 400 — некорректный id; 404 — канала нет или он выключен; 403 — нет подписки.
func (h *channelsHandler) playback(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "channel id must be a positive integer")
		return
	}

	channel, err := h.store.ActiveChannelByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "channel not found")
		return
	}
	if err != nil {
		h.internalError(w, "get channel", err)
		return
	}

	hasAccess, err := h.store.HasChannelAccess(r.Context(), user.ID, channel.ID, h.now())
	if err != nil {
		h.internalError(w, "check channel access", err)
		return
	}
	if !hasAccess {
		httpx.WriteError(w, http.StatusForbidden, "forbidden", "no active subscription for this channel")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, playbackResponse{
		ChannelID: channel.ID,
		URL:       h.streamBaseURL + channel.StreamPath,
	})
}

// user достаёт пользователя, которого положил middleware аутентификации.
func (h *channelsHandler) user(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		h.internalError(w, "user from context", errors.New("no authenticated user in request context"))
		return store.User{}, false
	}
	return u, true
}

// internalError пишет подробности в лог, а клиенту отдаёт общее сообщение.
func (h *channelsHandler) internalError(w http.ResponseWriter, op string, err error) {
	h.logger.Error("request failed", "op", op, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
}

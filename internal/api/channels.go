package api

import (
	"errors"
	"iptv/internal/httpx"
	"iptv/internal/store"
	"log/slog"
	"net/http"
	"strconv"
)

type channelsHandler struct {
	store         *store.Store
	streamBaseURL string
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

// list: GET /api/v1/channels
func (h *channelsHandler) list(w http.ResponseWriter, r *http.Request) {
	channels, err := h.store.ListActiveChannels(r.Context())
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

// playback: GET /api/v1/channels{id}/playback
func (h *channelsHandler) playback(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_id", "channel id must be a positive integer")
		return
	}

	channel, err := h.store.ActiveChannelsByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "channel not found")
		return
	}
	if err != nil {
		h.internalError(w, "get channel", err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, playbackResponse{
		ChannelID: channel.ID,
		URL:       h.streamBaseURL + channel.StreamPath,
	})
}

func (h *channelsHandler) internalError(w http.ResponseWriter, op string, err error) {
	h.logger.Error("request failed", "op", op, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

// Package httpx содержит общие хелперы для JSON-ответов и ошибок.
package httpx

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`    // машинно-читаемый код, например "unauthorized"
	Message string `json:"message"` // короткое сообщение для человека, без внутренних подробностей
}

// WriteJSON пишет v как JSON с заданным статусом.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// Заголовки уже отправлены: если клиент отвалился, сделать с ошибкой нечего.
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError пишет ошибку в едином формате: {"error": {"code": "...", "message": "..."}}.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

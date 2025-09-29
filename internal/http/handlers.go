package http

import (
	"encoding/json"
	"net/http"

	"github.com/Evgen-Mutagen/go-shortener-url/internal/service"
)

// Handlers HTTP обработчики
type Handlers struct {
	service *service.ShortenerService
}

// New создает новые HTTP обработчики
func New(service *service.ShortenerService) *Handlers {
	return &Handlers{
		service: service,
	}
}

// ShortenURLRequest запрос на сокращение URL
type ShortenURLRequest struct {
	URL string `json:"url"`
}

// ShortenURLResponse ответ на сокращение URL
type ShortenURLResponse struct {
	Result string `json:"result"`
}

// ShortenURL сокращает URL
func (h *Handlers) ShortenURL(w http.ResponseWriter, r *http.Request) {
	var req ShortenURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("User-ID")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusUnauthorized)
		return
	}

	result := h.service.ShortenURL(r.Context(), req.URL, userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.Status)

	if result.Error != "" {
		json.NewEncoder(w).Encode(map[string]string{"error": result.Error})
		return
	}

	json.NewEncoder(w).Encode(ShortenURLResponse{Result: result.ShortURL})
}

// ShortenURLJSON сокращает URL (JSON версия)
func (h *Handlers) ShortenURLJSON(w http.ResponseWriter, r *http.Request) {
	var req ShortenURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("User-ID")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusUnauthorized)
		return
	}

	result := h.service.ShortenURL(r.Context(), req.URL, userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.Status)

	if result.Error != "" {
		json.NewEncoder(w).Encode(map[string]string{"error": result.Error})
		return
	}

	json.NewEncoder(w).Encode(ShortenURLResponse{Result: result.ShortURL})
}

// RedirectURL перенаправляет на оригинальный URL
func (h *Handlers) RedirectURL(w http.ResponseWriter, r *http.Request, id string) {
	result := h.service.GetURL(r.Context(), id)

	if result.Error != "" {
		http.Error(w, result.Error, result.Status)
		return
	}

	if result.IsDeleted {
		http.Error(w, "URL deleted", http.StatusGone)
		return
	}

	http.Redirect(w, r, result.OriginalURL, http.StatusTemporaryRedirect)
}

// BatchItem элемент пакетного запроса
type BatchItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchRequest запрос на пакетное сокращение
type BatchRequest struct {
	Items []BatchItem `json:"items"`
}

// BatchResponseItem элемент ответа на пакетный запрос
type BatchResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// BatchResponse ответ на пакетное сокращение
type BatchResponse struct {
	Items []BatchResponseItem `json:"items"`
}

// ShortenURLBatch пакетное сокращение URL
func (h *Handlers) ShortenURLBatch(w http.ResponseWriter, r *http.Request) {
	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("User-ID")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusUnauthorized)
		return
	}

	items := make([]service.BatchItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = service.BatchItem{
			CorrelationID: item.CorrelationID,
			OriginalURL:   item.OriginalURL,
		}
	}

	result := h.service.ShortenURLBatch(r.Context(), items, userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.Status)

	if result.Error != "" {
		json.NewEncoder(w).Encode(map[string]string{"error": result.Error})
		return
	}

	responseItems := make([]BatchResponseItem, len(result.Items))
	for i, item := range result.Items {
		responseItems[i] = BatchResponseItem{
			CorrelationID: item.CorrelationID,
			ShortURL:      item.ShortURL,
		}
	}

	json.NewEncoder(w).Encode(BatchResponse{Items: responseItems})
}

// UserURL URL пользователя
type UserURL struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// GetUserURLsResponse ответ с URL пользователя
type GetUserURLsResponse struct {
	URLs []UserURL `json:"urls"`
}

// GetUserURLs получает все URL пользователя
func (h *Handlers) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("User-ID")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusUnauthorized)
		return
	}

	result := h.service.GetUserURLs(r.Context(), userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.Status)

	if result.Error != "" {
		json.NewEncoder(w).Encode(map[string]string{"error": result.Error})
		return
	}

	if result.Status == 204 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	urls := make([]UserURL, len(result.URLs))
	for i, url := range result.URLs {
		urls[i] = UserURL{
			ShortURL:    url.ShortURL,
			OriginalURL: url.OriginalURL,
		}
	}

	json.NewEncoder(w).Encode(GetUserURLsResponse{URLs: urls})
}

// DeleteUserURLsRequest запрос на удаление URL
type DeleteUserURLsRequest struct {
	URLIDs []string `json:"url_ids"`
}

// DeleteUserURLs помечает URL как удаленные
func (h *Handlers) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	var req DeleteUserURLsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("User-ID")
	if userID == "" {
		http.Error(w, "User ID required", http.StatusUnauthorized)
		return
	}

	result := h.service.DeleteUserURLs(r.Context(), userID, req.URLIDs)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.Status)

	if result.Error != "" {
		json.NewEncoder(w).Encode(map[string]string{"error": result.Error})
		return
	}
}

// Ping проверяет доступность сервиса
func (h *Handlers) Ping(w http.ResponseWriter, r *http.Request) {
	result := h.service.Ping(r.Context())

	w.WriteHeader(result.Status)

	if result.Error != "" {
		http.Error(w, result.Error, result.Status)
		return
	}
}

// GetStatsResponse ответ со статистикой
type GetStatsResponse struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}

// GetStats получает статистику сервиса
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	result := h.service.GetStats(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(result.Status)

	if result.Error != "" {
		json.NewEncoder(w).Encode(map[string]string{"error": result.Error})
		return
	}

	json.NewEncoder(w).Encode(GetStatsResponse{
		URLs:  result.URLs,
		Users: result.Users,
	})
}

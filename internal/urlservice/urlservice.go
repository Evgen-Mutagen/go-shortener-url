package urlservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/middleware"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/repository/postgres"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/util"
)

// URLService предоставляет методы для работы с сервисом сокращённых URL
type URLService struct {
	cfg       *configs.Config
	storage   *storage.Storage
	generator *util.IDGenerator
	Repo      *postgres.PostgresRepository
}

type BatchRequestItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// New создает новый экземпляр URLService
// Принимает:
//   - cfg: конфигурация сервиса
//   - storage: хранилище URL
//
// Возвращает:
//   - *URLService: инициализированный сервис
//   - error: ошибка инициализации
func New(cfg *configs.Config, storage *storage.Storage) (*URLService, error) {
	var repo *postgres.PostgresRepository
	var err error

	if cfg.DatabaseDSN != "" {
		repo, err = postgres.New(cfg.DatabaseDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to init database: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repo.InitTable(ctx); err != nil {
			return nil, fmt.Errorf("failed to init database table: %w", err)
		}
	}

	return &URLService{
		cfg:       cfg,
		storage:   storage,
		Repo:      repo,
		generator: util.NewIDGenerator(),
	}, nil
}

// Ping обрабатывает запрос проверки доступности сервиса
// Возвращает:
//   - 200 OK: если сервис доступен
//   - 500 Internal Server Error: если есть проблемы
func (s *URLService) Ping(w http.ResponseWriter, r *http.Request) {
	if s.Repo == nil {
		http.Error(w, "Database not configured", http.StatusInternalServerError)
		return
	}

	if err := s.Repo.Ping(r.Context()); err != nil {
		http.Error(w, "Database ping failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ShortenURL обрабатывает запрос на сокращение URL из plain text
// Формат запроса:
//
//	POST /
//	Content-Type: text/plain
//	Тело: оригинальный URL
//
// Возвращает:
//   - 201 Created: с сокращенным URL в теле
//   - 400 Bad Request: при неверном запросе
//   - 409 Conflict: если URL уже сокращен
func (s *URLService) ShortenURL(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ShortenURL started")
	defer fmt.Println("ShortenURL completed")
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	log.Printf("Shortening URL for user: %s", userID)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	url := string(body)
	var id string

	if s.Repo != nil {
		existingID, err := s.Repo.FindExistingURL(r.Context(), url)
		if err != nil {
			http.Error(w, "Ошибка проверки URL", http.StatusInternalServerError)
			return
		}
		if existingID != "" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID)))
			return
		}

		id = s.generator.Generate()
		if err := s.Repo.SaveURL(r.Context(), id, url, userID); err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
				existingID, err := s.Repo.FindExistingURL(r.Context(), url)
				if err != nil {
					http.Error(w, "Ошибка проверки URL", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte(fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID)))
				return
			}
			http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}

		if err := s.storage.Save(id, url, userID); err != nil && err != storage.ErrURLConflict {
			log.Printf("Failed to save URL to file storage: %v", err)
		}
	} else {
		id = s.generator.Generate()
		if err := s.storage.Save(id, url, userID); err != nil {
			if err == storage.ErrURLConflict {
				http.Error(w, "Conflict handling not implemented for file storage", http.StatusInternalServerError)
				return
			}
			http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}
	}

	log.Printf("Successfully saved URL: %s", url)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id)))
}

// RedirectURL выполняет перенаправление по сокращенному URL
// Формат запроса:
//
//	GET /{id}
//
// Возвращает:
//   - 307 Temporary Redirect: с Location на оригинальный URL
//   - 400 Bad Request: при неверном ID
//   - 410 Gone: если ссылка удалена
func (s *URLService) RedirectURL(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	var url string
	var isDeleted bool
	var exists bool

	if s.Repo != nil {
		originalURL, deleted, err := s.Repo.GetURL(r.Context(), id)
		if err != nil {
			http.Error(w, "Ошибка получения URL", http.StatusInternalServerError)
			return
		}
		url, exists, isDeleted = originalURL, originalURL != "", deleted
	} else {
		url, exists = s.storage.Get(id)
		isDeleted = false
	}

	if !exists {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	if isDeleted {
		w.WriteHeader(http.StatusGone)
		return
	}

	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// ShortenURLJSON обрабатывает JSON-запрос на сокращение URL
// Формат запроса:
//
//	POST /api/shorten
//	Content-Type: application/json
//	Тело: {"url": "оригинальный_URL"}
//
// Возвращает:
//   - 201 Created: {"result": "сокращенный_URL"}
//   - 400 Bad Request: при неверном JSON
//   - 409 Conflict: если URL уже сокращен
func (s *URLService) ShortenURLJSON(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}
	defer r.Body.Close()

	if req.URL == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "URL cannot be empty"})
		return
	}

	response := struct {
		Result string `json:"result"`
	}{}

	// Check if URL exists first
	var existingID string
	var err error
	if s.Repo != nil {
		existingID, err = s.Repo.FindExistingURL(r.Context(), req.URL)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Error checking URL"})
			return
		}
	}

	if existingID != "" {
		response.Result = fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(response)
		return
	}

	id := s.generator.Generate()
	if s.Repo != nil {
		if err := s.Repo.SaveURL(r.Context(), id, req.URL, userID); err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
				// If we get a conflict, try to find the existing URL again
				existingID, err := s.Repo.FindExistingURL(r.Context(), req.URL)
				if err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(map[string]string{"error": "Error checking URL"})
					return
				}
				response.Result = fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(response)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Error saving URL"})
			return
		}
	} else {
		if err := s.storage.Save(id, req.URL, userID); err != nil {
			if err == storage.ErrURLConflict {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{"error": "URL already exists"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Error saving URL"})
			return
		}
	}

	response.Result = fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, `{"error":"Error forming response"}`, http.StatusInternalServerError)
	}
}

// ShortenURLBatch обрабатывает пакетный запрос на сокращение URL
// Формат запроса:
//
//	POST /api/shorten/batch
//	Content-Type: application/json
//	Тело: [{"correlation_id": "id1", "original_url": "url1"}, ...]
//
// Возвращает:
//   - 201 Created: [{"correlation_id": "id1", "short_url": "short_url1"}, ...]
//   - 400 Bad Request: при неверном запросе
func (s *URLService) ShortenURLBatch(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var batch []BatchRequestItem
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(batch) == 0 {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, "Empty batch", http.StatusBadRequest)
		return
	}

	items := make(map[string]string)
	response := make([]BatchResponseItem, 0, len(batch))

	for _, item := range batch {
		if item.OriginalURL == "" {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, "URL cannot be empty", http.StatusBadRequest)
			return
		}
		id := s.generator.Generate()
		items[id] = item.OriginalURL
		response = append(response, BatchResponseItem{
			CorrelationID: item.CorrelationID,
			ShortURL:      fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id),
		})
	}

	if s.Repo != nil {
		ctx := r.Context()
		tx, err := s.Repo.BeginTx(ctx)
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}

		for id, url := range items {
			if err := tx.SaveURL(ctx, id, url, userID); err != nil {
				tx.Rollback()
				if err == storage.ErrURLConflict {
					existingID, err := s.Repo.FindExistingURL(ctx, url)
					if err != nil {
						http.Error(w, "Failed to check URL existence", http.StatusInternalServerError)
						return
					}
					// Обновляем response с существующим URL
					for i, item := range response {
						if item.ShortURL == fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id) {
							response[i].ShortURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID)
							break
						}
					}
					continue
				}
				http.Error(w, "Failed to save URL", http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
			return
		}
	} else {
		// Передаём userID в SaveBatch
		if err := s.storage.SaveBatch(items, userID); err != nil {
			http.Error(w, "Failed to save URLs", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetUserURLs возвращает все сокращенные URL пользователя
// Формат запроса:
//
//	GET /api/user/urls
//
// Возвращает:
//   - 200 OK: [{"short_url": "...", "original_url": "..."}, ...]
//   - 204 No Content: если URL отсутствуют
//   - 401 Unauthorized: если пользователь не аутентифицирован
func (s *URLService) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	log.Printf("GetUserURLs called with userID: %s", userID)
	if !ok || userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var urls map[string]string
	var err error

	if s.Repo != nil {
		log.Println("Checking database for user URLs")
		urls, err = s.Repo.GetUserURLs(r.Context(), userID)
		log.Printf("Found %d URLs in database", len(urls))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	} else {
		urls = make(map[string]string)
	}

	log.Printf("Checking file storage for user URLs")
	fileUrls := s.storage.GetUserURLs(userID)
	log.Printf("Found %d URLs in file storage", len(fileUrls))
	for k, v := range fileUrls {
		urls[k] = v
	}

	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	type urlPair struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}

	result := make([]urlPair, 0, len(urls))
	for id, original := range urls {
		result = append(result, urlPair{
			ShortURL:    fmt.Sprintf("%s%s", s.cfg.BaseURL, id),
			OriginalURL: original,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "Encoding error", http.StatusInternalServerError)
	}
}

// DeleteUserURLs помечает URL пользователя как удаленные (асинхронно)
// Формат запроса:
//
//	DELETE /api/user/urls
//	Тело: ["id1", "id2", ...]
//
// Возвращает:
//   - 202 Accepted: запрос принят в обработку
//   - 401 Unauthorized: если пользователь не аутентифицирован
func (s *URLService) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var urlIDs []string
	if err := json.NewDecoder(r.Body).Decode(&urlIDs); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	go func() {
		if s.Repo != nil {
			if err := s.Repo.MarkURLsAsDeleted(context.Background(), userID, urlIDs); err != nil {
				log.Printf("Failed to mark URLs as deleted: %v", err)
			}
		} else {
			log.Println("Delete operation not supported for file storage")
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}

// startDeletionWorker запускает фоновый worker для удаления URL
// Работает в отдельной горутине
// Обрабатывает URL пачками по batchSize или по таймауту timeout
func (s *URLService) startDeletionWorker() {
	const batchSize = 100
	const timeout = 1 * time.Second

	var (
		batch   []string
		userID  string
		timer   = time.NewTimer(timeout)
		batchCh = make(chan struct{})
	)
	defer timer.Stop()

	for {
		select {
		case <-batchCh:
			if len(batch) > 0 {
				if err := s.Repo.MarkURLsAsDeleted(context.Background(), userID, batch); err != nil {
					log.Printf("Failed to mark URLs as deleted: %v", err)
				}
				batch = batch[:0]
			}
		case <-timer.C:
			batchCh <- struct{}{}
			timer.Reset(timeout)
		}
	}
}

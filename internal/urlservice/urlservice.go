package urlservice

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/repository/postgres"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/util"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

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

func (s *URLService) ShortenURL(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ShortenURL started")
	defer fmt.Println("ShortenURL completed")
	userID, ok := r.Context().Value("userID").(string)
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

func (s *URLService) RedirectURL(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	var url string
	var exists bool

	if s.Repo != nil {
		originalURL, err := s.Repo.GetURL(r.Context(), id)
		if err != nil {
			http.Error(w, "Ошибка получения URL", http.StatusInternalServerError)
			return
		}
		url, exists = originalURL, originalURL != ""
	} else {
		url, exists = s.storage.Get(id)
	}

	if !exists {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (s *URLService) ShortenURLJSON(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
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

func (s *URLService) ShortenURLBatch(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := r.Context().Value("userID").(string)
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

func (s *URLService) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
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

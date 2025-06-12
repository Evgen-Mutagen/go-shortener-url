package urlservice

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/repository/postgres"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/util"
	"io"
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
	userID, ok := r.Context().Value("userID").(string)
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
		// Проверяем существование URL
		existingID, err := s.Repo.FindExistingURL(r.Context(), url)
		if err != nil {
			http.Error(w, "Ошибка проверки URL", http.StatusNotFound)
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
			if err == storage.ErrURLConflict {
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
	} else {
		id = s.generator.Generate()
		if err := s.storage.Save(id, url); err != nil {
			http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id)))
}

func (s *URLService) RedirectURL(w http.ResponseWriter, r *http.Request, id string) {
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var id string

	if s.Repo != nil {
		// Проверяем существование URL
		existingID, err := s.Repo.FindExistingURL(r.Context(), req.URL)
		if err != nil {
			http.Error(w, "Ошибка проверки URL", http.StatusInternalServerError)
			return
		}

		if existingID != "" {
			response := struct {
				Result string `json:"result"`
			}{
				Result: fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(response)
			return
		}

		id = s.generator.Generate()
		if err := s.Repo.SaveURL(r.Context(), id, req.URL, userID); err != nil {
			if err == storage.ErrURLConflict {
				existingID, err := s.Repo.FindExistingURL(r.Context(), req.URL)
				if err != nil {
					http.Error(w, "Ошибка проверки URL", http.StatusInternalServerError)
					return
				}
				response := struct {
					Result string `json:"result"`
				}{
					Result: fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID),
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(response)
				return
			}
			http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}
	} else {
		id = s.generator.Generate()
		if err := s.storage.Save(id, req.URL); err != nil {
			http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}
	}

	response := struct {
		Result string `json:"result"`
	}{
		Result: fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Ошибка при формировании ответа", http.StatusInternalServerError)
	}
}

func (s *URLService) ShortenURLBatch(w http.ResponseWriter, r *http.Request) {
	var batch []BatchRequestItem
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(batch) == 0 {
		http.Error(w, "Пустой batch", http.StatusBadRequest)
		return
	}

	items := make(map[string]string)
	response := make([]BatchResponseItem, 0, len(batch))

	for _, item := range batch {
		if item.OriginalURL == "" {
			http.Error(w, "URL не может быть пустым", http.StatusBadRequest)
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
			http.Error(w, "Ошибка начала транзакции", http.StatusInternalServerError)
			return
		}

		var txErr error
		defer func() {
			if txErr != nil {
				tx.Rollback()
			}
		}()

		for id, url := range items {
			if err := tx.SaveURL(ctx, id, url); err != nil {
				txErr = err
				http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
				return
			}
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Ошибка коммита транзакции", http.StatusInternalServerError)
			return
		}
	} else {
		if err := s.storage.SaveBatch(items); err != nil {
			http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Ошибка при формировании ответа", http.StatusInternalServerError)
	}
}

func (s *URLService) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(string)
	if !ok || userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var urls map[string]string
	var err error

	if s.Repo != nil {
		urls, err = s.Repo.GetUserURLs(r.Context(), userID)
	} else {
		urls = make(map[string]string)
	}

	if err != nil {
		http.Error(w, "Ошибка получения URL", http.StatusInternalServerError)
		return
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
			ShortURL:    fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id),
			OriginalURL: original,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "Ошибка при формировании ответа", http.StatusInternalServerError)
	}
}

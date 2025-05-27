package urlservice

import (
	"encoding/json"
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/repository/postgres"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/util"
	"io"
	"net/http"
	"strings"
)

type URLService struct {
	cfg       *configs.Config
	storage   *storage.Storage
	generator *util.IDGenerator
	Repo      *postgres.PostgresRepository
}

func New(cfg *configs.Config, storage *storage.Storage) (*URLService, error) {
	var repo *postgres.PostgresRepository
	var err error

	if cfg.DatabaseDSN != "" {
		repo, err = postgres.New(cfg.DatabaseDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to init database: %w", err)
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
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	url := string(body)
	id := s.generator.Generate()

	if err := s.storage.Save(id, url); err != nil {
		http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id)))
}

func (s *URLService) RedirectURL(w http.ResponseWriter, r *http.Request, id string) {
	url, exists := s.storage.Get(id)
	if !exists {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (s *URLService) ShortenURLJSON(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	id := s.generator.Generate()
	if err := s.storage.Save(id, req.URL); err != nil {
		http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
		return
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

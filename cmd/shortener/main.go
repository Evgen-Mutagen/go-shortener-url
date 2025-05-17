package main

import (
	"encoding/json"
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/compress"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/logger"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"sync"
)

var (
	urlStore  *storage.Storage
	Cfg       *configs.Config
	idCounter int
	idMutex   sync.Mutex
)

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

func main() {
	var err error
	Cfg, err = configs.LoadConfig()
	if err != nil {
		panic(err)
	}

	urlStore, err = storage.NewStorage(Cfg.FileStoragePath)
	if err != nil {
		panic(fmt.Errorf("не удалось инициализировать хранилище: %v", err))
	}

	loggerInstance, _ := zap.NewProduction()
	defer loggerInstance.Sync()

	r := chi.NewRouter()

	r.Use(compress.GzipCompress)

	r.Use(logger.WithLogging(loggerInstance))
	r.Post("/", shortenURL)
	r.Post("/api/shorten", ShortenURLJSON)
	r.Get("/{id}", redirectURL)

	fmt.Printf("Starting server on %s...\n", Cfg.ServerAddress)
	fmt.Printf("Using storage file: %s\n", Cfg.FileStoragePath)

	go func() {
		err := http.ListenAndServe(Cfg.ServerAddress, r)
		if err != nil {
			panic(err)
		}
	}()

	if extractHostPort(Cfg.BaseURL) != Cfg.ServerAddress {
		redirectAddr := extractHostPort(Cfg.BaseURL)
		fmt.Printf("Starting redirect server on %s...\n", redirectAddr)
		go func() {
			err := http.ListenAndServe(redirectAddr, r)
			if err != nil {
				panic(err)
			}
		}()
	}

	select {}
}

func extractHostPort(url string) string {

	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[:idx]
	}
	return url
}

func shortenURL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	url := string(body)

	id := generateID()

	if err := urlStore.Save(id, url); err != nil {
		http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("%s/%s", strings.TrimSuffix(Cfg.BaseURL, "/"), id)))
}

func redirectURL(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	url, exists := urlStore.Get(id)
	if !exists {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func ShortenURLJSON(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	id := generateID()
	if err := urlStore.Save(id, req.URL); err != nil {
		http.Error(w, "Ошибка сохранения URL", http.StatusInternalServerError)
		return
	}

	response := ShortenResponse{
		Result: fmt.Sprintf("%s/%s", strings.TrimSuffix(Cfg.BaseURL, "/"), id),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Ошибка при формировании ответа", http.StatusInternalServerError)
	}
}

func generateID() string {
	idMutex.Lock()
	defer idMutex.Unlock()
	idCounter++
	return fmt.Sprintf("%X", idCounter)
}

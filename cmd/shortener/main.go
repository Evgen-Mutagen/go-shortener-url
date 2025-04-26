package main

import (
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/cmd/config"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
)

var urlStore = make(map[string]string)
var idCounter = 0
var cfg *config.Config

func main() {
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		panic(err)
	}

	r := chi.NewRouter()

	r.Post("/", shortenURL)
	r.Get("/{id}", redirectURL)

	fmt.Printf("Starting server on %s...\n", cfg.ServerAddress)

	err = http.ListenAndServe(cfg.ServerAddress, r)
	if err != nil {
		panic(err)
	}
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
	urlStore[id] = url

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(cfg.BaseURL + id))
}

func redirectURL(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	url, exists := urlStore[id]
	if !exists {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func generateID() string {
	idCounter++
	return fmt.Sprintf("%X", idCounter)
}

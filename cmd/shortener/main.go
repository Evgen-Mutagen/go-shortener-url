package main

import (
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/cmd/config"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"strings"
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

	go func() {
		err := http.ListenAndServe(cfg.ServerAddress, r)
		if err != nil {
			panic(err)
		}
	}()

	if extractHostPort(cfg.BaseURL) != cfg.ServerAddress {
		redirectAddr := extractHostPort(cfg.BaseURL)
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
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	//
	url := string(body)
	http.Error(w, "q "+r.Host+" "+r.Method+" "+url, http.StatusBadRequest)
	//body, err := io.ReadAll(r.Body)
	//if err != nil || len(body) == 0 {
	//	http.Error(w, "Некорректный запрос", http.StatusBadRequest)
	//	return
	//}
	//
	//defer r.Body.Close()
	//
	//url := string(body)
	//
	//id := generateID()
	//urlStore[id] = url
	//
	//w.WriteHeader(http.StatusCreated)
	//w.Header().Set("Content-Type", "text/plain")
	//w.Write([]byte(cfg.BaseURL + id))
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

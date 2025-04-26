package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
)

var urlStore = make(map[string]string)
var idCounter = 0

func main() {
	r := chi.NewRouter()

	r.Post("/", shortenURL)
	r.Get("/{id}", redirectURL)

	err := http.ListenAndServe(`:8080`, r)
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
	w.Write([]byte("http://localhost:8080/" + id))
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

package main

import (
	"fmt"
	"io"
	"net/http"
)

var urlStore = make(map[string]string)
var idCounter = 0

func main() {
	http.HandleFunc("/", handleURL)

	err := http.ListenAndServe(`:8080`, nil)
	if err != nil {
		panic(err)
	}
}

func handleURL(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		shortenURL(w, r)
	} else if r.Method == http.MethodGet {
		redirectURL(w, r)
	} else {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
	}
}

func shortenURL(w http.ResponseWriter, r *http.Request) {

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Некорректный запрос", http.StatusBadRequest)
		return
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
		}
	}(r.Body)

	url := string(body)

	id := generateID()
	urlStore[id] = url

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("http://localhost:8080/" + id))
}

func redirectURL(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/"):]

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

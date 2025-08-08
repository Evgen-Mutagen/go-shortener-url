package urlservice_test

import (
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/urlservice"
	"net/http/httptest"
	"strings"
)

func ExampleURLService_ShortenURL() {
	// Инициализация сервиса
	cfg := &configs.Config{
		ServerAddress:   "localhost:8080",
		BaseURL:         "http://localhost:8080/",
		FileStoragePath: "",
	}
	s, _ := storage.NewStorage("")
	service, _ := urlservice.New(cfg, s)

	// Создание тестового запроса
	req := httptest.NewRequest("POST", "/", strings.NewReader("https://example.com"))
	w := httptest.NewRecorder()

	// Вызов метода
	service.ShortenURL(w, req)

	// Получение ответа
	resp := w.Result()
	defer resp.Body.Close()
	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("Content-Type"))

	// Вывод:
	// 201
	// text/plain
}

func ExampleURLService_RedirectURL() {
	// Инициализация сервиса с тестовыми данными
	cfg := &configs.Config{
		ServerAddress:   "localhost:8080",
		BaseURL:         "http://localhost:8080/",
		FileStoragePath: "",
	}
	s, _ := storage.NewStorage("")
	service, _ := urlservice.New(cfg, s)
	s.Save("abc123", "https://example.com", "user1")

	// Создание тестового запроса
	req := httptest.NewRequest("GET", "/abc123", nil)
	w := httptest.NewRecorder()

	// Вызов метода
	service.RedirectURL(w, req, "abc123")

	// Получение ответа
	resp := w.Result()
	defer resp.Body.Close()
	fmt.Println(resp.StatusCode)
	fmt.Println(resp.Header.Get("Location"))

	// Вывод:
	// 307
	// https://example.com
}

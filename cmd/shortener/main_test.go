package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/middleware"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/urlservice"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupTestService(t *testing.T) (*urlservice.URLService, *storage.Storage) {
	tmpFile, err := os.CreateTemp("", "test_storage_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	cfg := &configs.Config{
		ServerAddress:   "localhost:8080",
		BaseURL:         "http://localhost:8080/",
		FileStoragePath: tmpFile.Name(),
	}

	storage, err := storage.NewStorage(cfg.FileStoragePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	service, err := urlservice.New(cfg, storage)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})

	return service, storage
}

type contextKey string

func createRequestWithUserID(method, url string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, url, body)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "test-user-id")
	return req.WithContext(ctx)
}

func Test_redirectURL(t *testing.T) {
	service, storage := setupTestService(t)

	id := "testID123"
	originalURL := "https://google.com"
	userID := "123"

	// Явно сохраняем тестовый URL
	if err := storage.Save(id, originalURL, userID); err != nil {
		t.Fatalf("Failed to save test URL: %v", err)
	}

	// Проверяем, что URL сохранился
	storedURL, exists := storage.Get(id)
	if !exists || storedURL != originalURL {
		t.Fatalf("Test URL not found in storage")
	}

	tests := []struct {
		name             string
		id               string
		expectedCode     int
		expectedLocation string
	}{
		{
			name:             "Valid ID",
			id:               id,
			expectedCode:     http.StatusTemporaryRedirect,
			expectedLocation: originalURL,
		},
		{
			name:         "Invalid ID",
			id:           "invalid-id",
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.id, nil)

			// Правильно устанавливаем параметр маршрута
			routeContext := chi.NewRouteContext()
			routeContext.URLParams.Add("id", tt.id)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))

			w := httptest.NewRecorder()

			service.RedirectURL(w, req, tt.id)

			if w.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, w.Code)
			}

			if tt.expectedLocation != "" {
				if location := w.Header().Get("Location"); location != tt.expectedLocation {
					t.Errorf("Expected location %q, got %q", tt.expectedLocation, location)
				}
			}
		})
	}
}

func Test_shortenURL(t *testing.T) {
	service, _ := setupTestService(t)

	tests := []struct {
		name                   string
		body                   string
		expectedCode           int
		expectLocationContains string
	}{
		{
			name:                   "Valid URL",
			body:                   "https://google.com",
			expectedCode:           http.StatusCreated,
			expectLocationContains: "http://localhost:8080/",
		},
		{
			name:         "Empty body",
			body:         "",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:                   "Invalid request body",
			body:                   "invalid-url",
			expectedCode:           http.StatusCreated,
			expectLocationContains: "http://localhost:8080/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createRequestWithUserID(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			service.ShortenURL(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			if tt.expectedCode == http.StatusCreated {
				location := w.Body.String()
				assert.Contains(t, location, tt.expectLocationContains)
			}
		})
	}
}

func Test_shortenURLJSON(t *testing.T) {
	service, _ := setupTestService(t)

	tests := []struct {
		name                 string
		body                 string
		expectedCode         int
		expectedContentType  string
		expectResultContains string
	}{
		{
			name:                 "Valid URL",
			body:                 `{"url":"https://google.com"}`,
			expectedCode:         http.StatusCreated,
			expectedContentType:  "application/json",
			expectResultContains: "http://localhost:8080/",
		},
		{
			name:         "Empty body",
			body:         `{}`,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createRequestWithUserID(http.MethodPost, "/api/shorten", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			service.ShortenURLJSON(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			if tt.expectedCode == http.StatusCreated {
				var response struct {
					Result string `json:"result"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Result, tt.expectResultContains)
				assert.Equal(t, tt.expectedContentType, w.Header().Get("Content-Type"))
			}
		})
	}
}

func Test_pingHandler(t *testing.T) {
	service, _ := setupTestService(t)

	req := createRequestWithUserID(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	service.Ping(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func Test_shortenURLBatch(t *testing.T) {
	service, _ := setupTestService(t)

	tests := []struct {
		name                 string
		body                 string
		expectedCode         int
		expectedContentType  string
		expectItemsCount     int
		expectResultContains string
	}{
		{
			name: "Valid batch",
			body: `[
                {"correlation_id": "1", "original_url": "https://google.com"},
                {"correlation_id": "2", "original_url": "https://yandex.ru"}
            ]`,
			expectedCode:         http.StatusCreated,
			expectedContentType:  "application/json",
			expectItemsCount:     2,
			expectResultContains: "http://localhost:8080/",
		},
		{
			name:         "Empty batch",
			body:         `[]`,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Invalid JSON",
			body:         `invalid`,
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Empty URL",
			body:         `[{"correlation_id": "1", "original_url": ""}]`,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createRequestWithUserID(http.MethodPost, "/api/shorten/batch", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			service.ShortenURLBatch(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			if tt.expectedCode == http.StatusCreated {
				var response []struct {
					CorrelationID string `json:"correlation_id"`
					ShortURL      string `json:"short_url"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectItemsCount, len(response))
				for _, item := range response {
					assert.Contains(t, item.ShortURL, tt.expectResultContains)
				}
				assert.Equal(t, tt.expectedContentType, w.Header().Get("Content-Type"))
			}
		})
	}
}

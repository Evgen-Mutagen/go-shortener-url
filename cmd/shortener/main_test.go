package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_redirectURL(t *testing.T) {
	urlStore = make(map[string]string)
	id := generateID()
	urlStore[id] = "https://google.com"

	tests := []struct {
		name             string
		id               string
		expectedCode     int
		expectedLocation string
	}{
		{
			name: "Valid ID", id: id, expectedCode: http.StatusTemporaryRedirect, expectedLocation: "https://google.com",
		},
		{
			name: "Invalid ID", id: "invalid-id", expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.id, nil)

			routeContext := chi.NewRouteContext()
			routeContext.URLParams.Add("id", tt.id)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))

			w := httptest.NewRecorder()

			redirectURL(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			if tt.expectedCode == http.StatusTemporaryRedirect {
				assert.Equal(t, tt.expectedLocation, w.Header().Get("Location"))
			}
		})
	}
}

func Test_shortenURL(t *testing.T) {
	Cfg = &configs.Config{
		ServerAddress: "localhost:8080",
		BaseURL:       "http://localhost:8080/",
	}

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
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			shortenURL(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			if tt.expectedCode == http.StatusCreated {
				location := w.Body.String()
				assert.Contains(t, location, tt.expectLocationContains)
			}
		})
	}
}

func Test_shortenURLJSON(t *testing.T) {
	Cfg = &configs.Config{
		ServerAddress: "localhost:8080",
		BaseURL:       "http://localhost:8080/",
	}

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
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			ShortenURLJSON(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			if tt.expectedCode == http.StatusCreated {
				var response ShortenResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response.Result, tt.expectResultContains)
				assert.Equal(t, tt.expectedContentType, w.Header().Get("Content-Type"))
			}
		})
	}
}

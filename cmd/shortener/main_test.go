package main

import (
	"bytes"
	"context"
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
		name         string
		id           string
		expectedCode int
	}{
		{
			name: "Valid ID", id: id, expectedCode: http.StatusTemporaryRedirect,
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
				assert.Equal(t, "https://google.com", w.Header().Get("Location"))
			}
		})
	}
}

func Test_shortenURL(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		expectedCode int
	}{
		{
			name:         "Valid URL",
			body:         "https://google.com",
			expectedCode: http.StatusCreated,
		},
		{
			name:         "Empty body",
			body:         "",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Invalid request body",
			body:         "invalid-url",
			expectedCode: http.StatusCreated,
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
				assert.Contains(t, location, "http://localhost:8080/")
			}
		})
	}
}

package main

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_handleURL(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		body         string
		expectedCode int
	}{
		{
			name:         "POST valid URL",
			method:       http.MethodPost,
			body:         "https://google.com",
			expectedCode: http.StatusCreated,
		},
		{
			name:         "GET valid ID",
			method:       http.MethodGet,
			body:         "",
			expectedCode: http.StatusTemporaryRedirect,
		},
		{
			name:         "Unsupported method",
			method:       http.MethodPut,
			body:         "",
			expectedCode: http.StatusMethodNotAllowed,
		},
	}

	var validID string

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			var w = httptest.NewRecorder()

			if tt.method == http.MethodPost {
				req = httptest.NewRequest(tt.method, "/", bytes.NewBufferString(tt.body))
				handleURL(w, req)
				assert.Equal(t, tt.expectedCode, w.Code)
				if tt.expectedCode == http.StatusCreated {
					location := w.Body.String()
					assert.Contains(t, location, "http://localhost:8080/")
					validID = extractIDFromLocation(location)
				}
			} else if tt.method == http.MethodGet {
				if validID == "" {
					req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("https://google.com"))
					handleURL(w, req)
					location := w.Body.String()
					validID = extractIDFromLocation(location)
				}
				req = httptest.NewRequest(tt.method, "/"+validID, nil)
				handleURL(w, req)
				assert.Equal(t, tt.expectedCode, w.Code)
				if tt.expectedCode == http.StatusTemporaryRedirect {
					assert.Equal(t, "https://google.com", w.Header().Get("Location"))
				}
			} else {
				req = httptest.NewRequest(tt.method, "/", nil)
				handleURL(w, req)
				assert.Equal(t, tt.expectedCode, w.Code)
			}
		})
	}
}

func extractIDFromLocation(location string) string {
	return location[len("http://localhost:8080/"):]
}

func Test_redirectURL(t *testing.T) {
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

package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
)

const (
	cookieName    = "user_id"
	secretKey     = "your-secret-key"
	cookieExpires = 24 * time.Hour * 30
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil || !validateCookie(cookie) {
			userID := uuid.New().String()
			setAuthCookie(w, userID)
			w.Header().Set("Authorization", "Bearer "+userID)
			ctx := context.WithValue(r.Context(), "userID", userID)
			r = r.WithContext(ctx)
		} else {
			w.Header().Set("Authorization", "Bearer "+cookie.Value)
			ctx := context.WithValue(r.Context(), "userID", cookie.Value)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func validateCookie(cookie *http.Cookie) bool {
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return false
	}
	return parts[1] == generateSignature(parts[0]) // Проверка HMAC подписи
}

func setAuthCookie(w http.ResponseWriter, userID string) {
	signature := generateSignature(userID)
	value := userID + "." + signature

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(cookieExpires),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

func generateSignature(data string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// WithLogging middleware для логирования запросов
func WithLogging(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			lrw := &loggingResponseWriter{ResponseWriter: w}

			next.ServeHTTP(lrw, r)

			duration := time.Since(start)

			logger.Info("Request",
				zap.String("method", r.Method),
				zap.String("uri", r.RequestURI),
				zap.Int("status", lrw.status),
				zap.Int("size", lrw.size),
				zap.Duration("duration", duration),
			)
		})
	}
}

// loggingResponseWriter оборачивает ResponseWriter для логирования
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

// WriteHeader перехватывает статус код ответа
func (lrw *loggingResponseWriter) WriteHeader(status int) {
	lrw.status = status
	lrw.ResponseWriter.WriteHeader(status)
}

// Write перехватывает запись
func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := lrw.ResponseWriter.Write(b)
	lrw.size += size
	return size, err
}

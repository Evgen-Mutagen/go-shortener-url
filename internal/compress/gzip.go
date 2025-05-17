package compress

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(nil)
	},
}

func GzipCompress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer gz.Close()
			r.Body = gz
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gzw := &gzipResponseWriter{ResponseWriter: w}
		defer gzw.Close()

		contentType := r.Header.Get("Content-Type")
		if contentType == "" {
			contentType = http.DetectContentType([]byte{})
		}

		if strings.Contains(contentType, "application/json") ||
			strings.Contains(contentType, "text/html") ||
			strings.Contains(contentType, "text/plain") {
			gzw.Header().Set("Content-Encoding", "gzip")
			next.ServeHTTP(gzw, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz  *gzip.Writer
	buf *bytes.Buffer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if w.gz == nil {
		return w.ResponseWriter.Write(b)
	}
	return w.gz.Write(b)
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.gz == nil && strings.Contains(w.Header().Get("Content-Encoding"), "gzip") {
		w.buf = &bytes.Buffer{}
		w.gz = gzipPool.Get().(*gzip.Writer)
		w.gz.Reset(w.buf)
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Close() {
	if w.gz != nil {
		w.gz.Close()
		gzipPool.Put(w.gz)
		if w.buf != nil {
			w.ResponseWriter.Write(w.buf.Bytes())
		}
	}
}

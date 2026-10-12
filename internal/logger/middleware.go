package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

func RequestLogger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			lw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(lw, r)

			log.Info("request handled",
				zap.String("uri", r.RequestURI),
				zap.String("method", r.Method),
				zap.Duration("duration", time.Since(start)),
				zap.Int("status", lw.status),
				zap.Int("size", lw.size),
			)
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter

	status      int
	size        int
	wroteHeader bool
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true

	n, err := w.ResponseWriter.Write(b)
	w.size += n

	return n, err
}

func (w *loggingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

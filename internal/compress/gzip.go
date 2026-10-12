package compress

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var writerPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch enc := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding"))); enc {
		case "", "identity":
		case "gzip":
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "invalid gzip body", http.StatusBadRequest)

				return
			}

			r.Body = &gzipReadCloser{Reader: zr, body: r.Body}
			r.Header.Del("Content-Encoding")
			r.Header.Del("Content-Length")
			r.ContentLength = -1
		default:
			http.Error(w, "unsupported Content-Encoding "+strconv.Quote(enc), http.StatusUnsupportedMediaType)

			return
		}

		if !acceptsGzip(r) {
			next.ServeHTTP(w, r)

			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()

		next.ServeHTTP(gw, r)
	})
}

func acceptsGzip(r *http.Request) bool {
	for _, header := range r.Header.Values("Accept-Encoding") {
		for _, part := range strings.Split(header, ",") {
			name, params, _ := strings.Cut(part, ";")
			if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
				continue
			}

			q, found := strings.CutPrefix(strings.TrimSpace(params), "q=")
			if !found {
				return true
			}

			if v, err := strconv.ParseFloat(q, 64); err == nil && v > 0 {
				return true
			}
		}
	}

	return false
}

func compressible(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")

	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "application/json", "text/html":
		return true
	default:
		return false
	}
}

type gzipReadCloser struct {
	*gzip.Reader
	body io.ReadCloser
}

func (r *gzipReadCloser) Close() error {
	zerr := r.Reader.Close()
	if err := r.body.Close(); err != nil {
		return err
	}

	return zerr
}

type gzipResponseWriter struct {
	http.ResponseWriter

	gz         *gzip.Writer
	status     int
	headerSent bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.headerSent || status < http.StatusOK {
		w.ResponseWriter.WriteHeader(status)

		return
	}

	if w.status == 0 {
		w.status = status
	}
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.headerSent {
		w.sendHeader(b)
	}

	if w.gz != nil {
		return w.gz.Write(b)
	}

	return w.ResponseWriter.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if !w.headerSent {
		w.sendHeader(nil)
	}

	if w.gz != nil {
		_ = w.gz.Flush()
	}

	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *gzipResponseWriter) sendHeader(firstChunk []byte) {
	w.headerSent = true

	status := w.status
	if status == 0 {
		status = http.StatusOK
	}

	h := w.Header()

	if h.Get("Content-Type") == "" && len(firstChunk) > 0 {
		h.Set("Content-Type", http.DetectContentType(firstChunk))
	}

	if hasBody(status) && h.Get("Content-Encoding") == "" && compressible(h.Get("Content-Type")) {
		h.Set("Content-Encoding", "gzip")
		h.Del("Content-Length")
		h.Add("Vary", "Accept-Encoding")

		w.gz = writerPool.Get().(*gzip.Writer)
		w.gz.Reset(w.ResponseWriter)
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) close() {
	if !w.headerSent && w.status != 0 {
		w.headerSent = true
		w.ResponseWriter.WriteHeader(w.status)
	}

	if w.gz == nil {
		return
	}

	_ = w.gz.Close()

	w.gz.Reset(io.Discard)
	writerPool.Put(w.gz)
	w.gz = nil
}

func hasBody(status int) bool {
	return status != http.StatusNoContent && status != http.StatusNotModified
}

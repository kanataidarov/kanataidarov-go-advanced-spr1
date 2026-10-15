package compress

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func gzipBytes(t *testing.T, data string) []byte {
	t.Helper()

	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func gunzip(t *testing.T, data []byte) string {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("response is not gzip: %v", err)
	}

	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("cannot decompress response: %v", err)
	}

	return string(out)
}

func echoBody(seen *http.Header) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*seen = r.Header.Clone()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(body)
	}
}

func TestGzipDecompressesRequest(t *testing.T) {
	const payload = `{"id":"Alloc","type":"gauge","value":12.5}`

	var seen http.Header

	req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(gzipBytes(t, payload)))
	req.Header.Set("Content-Encoding", "gzip")

	rec := httptest.NewRecorder()
	Gzip(echoBody(&seen)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if rec.Body.String() != payload {
		t.Errorf("handler got body %q, want %q", rec.Body.String(), payload)
	}

	if enc := seen.Get("Content-Encoding"); enc != "" {
		t.Errorf("handler still sees Content-Encoding %q", enc)
	}
}

func TestGzipPassesPlainRequest(t *testing.T) {
	var seen http.Header

	for _, enc := range []string{"", "identity"} {
		req := httptest.NewRequest(http.MethodPost, "/update/", strings.NewReader("plain"))
		if enc != "" {
			req.Header.Set("Content-Encoding", enc)
		}

		rec := httptest.NewRecorder()
		Gzip(echoBody(&seen)).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK || rec.Body.String() != "plain" {
			t.Errorf("Content-Encoding %q: got %d %q, want 200 \"plain\"", enc, rec.Code, rec.Body.String())
		}
	}
}

func TestGzipRejectsBadRequests(t *testing.T) {
	tests := []struct {
		name       string
		encoding   string
		body       string
		wantStatus int
	}{
		{name: "not gzip data", encoding: "gzip", body: "definitely not gzip", wantStatus: http.StatusBadRequest},
		{name: "unsupported encoding", encoding: "br", body: "x", wantStatus: http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

			req := httptest.NewRequest(http.MethodPost, "/update/", strings.NewReader(tt.body))
			req.Header.Set("Content-Encoding", tt.encoding)

			rec := httptest.NewRecorder()
			Gzip(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			if called {
				t.Error("next handler must not be called")
			}
		})
	}
}

func TestGzipCompressesResponse(t *testing.T) {
	const htmlPage = "<!DOCTYPE html><html><body>metrics</body></html>"

	tests := []struct {
		name           string
		acceptEncoding string
		contentType    string
		body           string
		wantCompressed bool
		wantType       string
	}{
		{name: "json", acceptEncoding: "gzip", contentType: "application/json", body: `{"id":"a"}`, wantCompressed: true},
		{name: "html with charset", acceptEncoding: "gzip, deflate, br", contentType: "text/html; charset=utf-8", body: htmlPage, wantCompressed: true},
		{name: "sniffed html", acceptEncoding: "gzip", body: htmlPage, wantCompressed: true, wantType: "text/html; charset=utf-8"},
		{name: "upper case and q", acceptEncoding: "deflate;q=1.0, GZIP;q=0.5", contentType: "application/json", body: `{}`, wantCompressed: true},
		{name: "plain text is not compressed", acceptEncoding: "gzip", contentType: "text/plain; charset=utf-8", body: "12.5"},
		{name: "client without gzip", acceptEncoding: "deflate", contentType: "application/json", body: `{"id":"a"}`},
		{name: "gzip explicitly refused", acceptEncoding: "gzip;q=0", contentType: "application/json", body: `{"id":"a"}`},
		{name: "no Accept-Encoding", contentType: "text/html", body: htmlPage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}

				w.Header().Set("Content-Length", "999")
				w.WriteHeader(http.StatusCreated)

				_, _ = io.WriteString(w, tt.body)
			})

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}

			rec := httptest.NewRecorder()
			Gzip(next).ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Errorf("got status %d, want 201", rec.Code)
			}

			if tt.wantType != "" && rec.Header().Get("Content-Type") != tt.wantType {
				t.Errorf("got Content-Type %q, want %q", rec.Header().Get("Content-Type"), tt.wantType)
			}

			if !tt.wantCompressed {
				if enc := rec.Header().Get("Content-Encoding"); enc != "" {
					t.Errorf("got Content-Encoding %q, want none", enc)
				}

				if rec.Body.String() != tt.body {
					t.Errorf("got body %q, want %q", rec.Body.String(), tt.body)
				}

				return
			}

			if enc := rec.Header().Get("Content-Encoding"); enc != "gzip" {
				t.Fatalf("got Content-Encoding %q, want gzip", enc)
			}

			if v := rec.Header().Get("Vary"); v != "Accept-Encoding" {
				t.Errorf("got Vary %q, want Accept-Encoding", v)
			}

			if cl := rec.Header().Get("Content-Length"); cl != "" {
				t.Errorf("got Content-Length %q, want it removed", cl)
			}

			if got := gunzip(t, rec.Body.Bytes()); got != tt.body {
				t.Errorf("got decompressed body %q, want %q", got, tt.body)
			}
		})
	}
}

func TestGzipSkipsResponsesWithoutBody(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	Gzip(next).ServeHTTP(rec, req)

	if enc := rec.Header().Get("Content-Encoding"); enc != "" {
		t.Errorf("got Content-Encoding %q for 204, want none", enc)
	}

	if rec.Body.Len() != 0 {
		t.Errorf("got body %q for 204, want empty", rec.Body.String())
	}
}

func TestGzipRoundTrip(t *testing.T) {
	srv := httptest.NewServer(Gzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})))
	defer srv.Close()

	const payload = `{"id":"PollCount","type":"counter","delta":5}`

	compressed := gzipBytes(t, payload)

	var wg sync.WaitGroup

	for range 20 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(compressed))
			if err != nil {
				t.Error(err)

				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Error(err)

				return
			}
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Error(err)

				return
			}

			if !resp.Uncompressed {
				t.Error("response was not gzip-compressed by the server")
			}

			if string(body) != payload {
				t.Errorf("got %q, want %q", body, payload)
			}
		}()
	}

	wg.Wait()
}

func TestGzipStatusWithoutBodyIsNotCompressed(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Accept-Encoding", "gzip")

	rec := httptest.NewRecorder()
	Gzip(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("got status %d, want 202", rec.Code)
	}

	if enc := rec.Header().Get("Content-Encoding"); enc != "" || rec.Body.Len() != 0 {
		t.Errorf("got Content-Encoding %q and %d body bytes, want plain empty response", enc, rec.Body.Len())
	}
}

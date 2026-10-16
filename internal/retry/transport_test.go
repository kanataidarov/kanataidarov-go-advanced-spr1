package retry

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type consumingTransport struct {
	failures int
	bodies   []string
	next     http.RoundTripper
}

func (c *consumingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	c.bodies = append(c.bodies, string(body))

	if len(c.bodies) <= c.failures {
		return nil, errTemporary
	}

	fresh := r.Clone(r.Context())
	fresh.Body = io.NopCloser(bytes.NewReader(body))

	return c.next.RoundTrip(fresh)
}

func newEchoServer(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestTransportReplaysBodyOnRetry(t *testing.T) {
	srv := newEchoServer(t)
	p, waited := testPolicy()
	inner := &consumingTransport{failures: 2, next: http.DefaultTransport}
	client := &http.Client{Transport: &Transport{Next: inner, Policy: p}}

	resp, err := client.Post(srv.URL, "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	echoed, _ := io.ReadAll(resp.Body)
	if string(echoed) != "payload" {
		t.Errorf("server got body %q, want %q", echoed, "payload")
	}

	if len(inner.bodies) != 3 || len(*waited) != 2 {
		t.Fatalf("got %d attempts and %d retries, want 3 and 2", len(inner.bodies), len(*waited))
	}

	for i, body := range inner.bodies {
		if body != "payload" {
			t.Errorf("attempt %d sent body %q, want %q", i+1, body, "payload")
		}
	}
}

func TestTransportSendsUnrewindableBodyOnce(t *testing.T) {
	p, waited := testPolicy()
	inner := &consumingTransport{failures: 1}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://example.invalid",
		io.NopCloser(strings.NewReader("payload")))
	if err != nil {
		t.Fatal(err)
	}

	if req.GetBody != nil {
		t.Fatal("test request body must not be rewindable")
	}

	resp, err := (&Transport{Next: inner, Policy: p}).RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}

	if err == nil || len(inner.bodies) != 1 || len(*waited) != 0 {
		t.Errorf("got error %v, %d attempts and %d retries, want an error, 1 and 0",
			err, len(inner.bodies), len(*waited))
	}
}

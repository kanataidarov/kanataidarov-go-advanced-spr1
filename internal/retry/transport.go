package retry

import (
	"context"
	"net/http"
)

type Transport struct {
	Next   http.RoundTripper
	Policy Policy
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.Next
	if next == nil {
		next = http.DefaultTransport
	}

	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		return next.RoundTrip(req)
	}

	attempt := 0

	return DoValue(req.Context(), t.Policy, func(context.Context) (*http.Response, error) {
		attempt++

		r, err := rewind(req, attempt)
		if err != nil {
			return nil, err
		}

		return next.RoundTrip(r)
	})
}

func rewind(req *http.Request, attempt int) (*http.Request, error) {
	if attempt == 1 || req.GetBody == nil {
		return req, nil
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	r := req.Clone(req.Context())
	r.Body = body

	return r, nil
}

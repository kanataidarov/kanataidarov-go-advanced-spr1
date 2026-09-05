package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

const updatePrefix = "update"

type MetricsHandler struct {
	metrics *service.MetricsService
}

func NewMetricsHandler(metrics *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{metrics: metrics}
}

func (h *MetricsHandler) Router() http.Handler {
	return http.HandlerFunc(h.route)
}

func (h *MetricsHandler) route(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)

	if len(parts) == 0 || parts[0] != updatePrefix {
		http.Error(w, "unknown endpoint", http.StatusNotFound)

		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "only POST is allowed", http.StatusMethodNotAllowed)

		return
	}

	h.update(w, parts)
}

func (h *MetricsHandler) update(w http.ResponseWriter, parts []string) {
	if len(parts) != 4 {
		http.Error(w, "metric name is not specified", http.StatusNotFound)

		return
	}

	mType, name, value := parts[1], parts[2], parts[3]

	if err := h.metrics.Update(mType, name, value); err != nil {
		switch {
		case errors.Is(err, service.ErrEmptyName):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, service.ErrUnknownType), errors.Is(err, service.ErrInvalidValue):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("metric saved")); err != nil {
		return
	}
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "/")
}

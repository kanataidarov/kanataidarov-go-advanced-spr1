package handler

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mailru/easyjson"
	"go.uber.org/zap"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

type metricView struct {
	Name  string
	Type  string
	Value string
}

var indexTemplate = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Metrics</title>
</head>
<body>
<h1>Metrics</h1>
<table>
<thead><tr><th>Name</th><th>Type</th><th>Value</th></tr></thead>
<tbody>
{{range .}}<tr><td>{{.Name}}</td><td>{{.Type}}</td><td>{{.Value}}</td></tr>
{{end}}</tbody>
</table>
</body>
</html>
`))

type MetricsService interface {
	Update(ctx context.Context, mType, name, rawValue string) error
	Value(ctx context.Context, mType, name string) (string, error)
	UpdateMetric(ctx context.Context, m models.Metrics) (models.Metrics, error)
	Metric(ctx context.Context, mType, name string) (models.Metrics, error)
	All(ctx context.Context) ([]models.Metrics, error)
	UpdateMetrics(ctx context.Context, metrics []models.Metrics) error
}

type Pinger interface {
	Ping(ctx context.Context) error
}

const pingTimeout = 3 * time.Second

type MetricsHandler struct {
	metrics MetricsService
	pinger  Pinger
	log     *zap.Logger
}

type Option func(*MetricsHandler)

func WithPinger(p Pinger) Option {
	return func(h *MetricsHandler) {
		h.pinger = p
	}
}

func NewMetricsHandler(metrics MetricsService, log *zap.Logger, opts ...Option) *MetricsHandler {
	h := &MetricsHandler{metrics: metrics, log: log}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

func (h *MetricsHandler) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.StripSlashes)

	r.Get("/", h.index)
	r.Get("/ping", h.ping)
	r.Post("/updates", h.updatesJSON)

	r.Route("/update", func(r chi.Router) {
		r.Post("/", h.updateJSON)
		r.Post("/{type}/{name}/{value}", h.update)
		r.Post("/{type}", notFound)
		r.Post("/{type}/{name}", notFound)
	})

	r.Route("/value", func(r chi.Router) {
		r.Post("/", h.valueJSON)
		r.Get("/{type}/{name}", h.value)
		r.Get("/", notFound)
		r.Get("/{type}", notFound)
	})

	return r
}

func (h *MetricsHandler) update(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")
	value := chi.URLParam(r, "value")

	if err := h.metrics.Update(r.Context(), mType, name, value); err != nil {
		h.writeServiceError(w, err)

		return
	}

	writePlain(w, http.StatusOK, "metric saved")
}

func (h *MetricsHandler) value(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")

	value, err := h.metrics.Value(r.Context(), mType, name)
	if err != nil {
		h.writeServiceError(w, err)

		return
	}

	writePlain(w, http.StatusOK, value)
}

func (h *MetricsHandler) updateJSON(w http.ResponseWriter, r *http.Request) {
	var req models.Metrics
	if err := easyjson.UnmarshalFromReader(r.Body, &req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)

		return
	}

	updated, err := h.metrics.UpdateMetric(r.Context(), req)
	if err != nil {
		h.writeServiceError(w, err)

		return
	}

	h.writeJSON(w, http.StatusOK, updated)
}

func (h *MetricsHandler) updatesJSON(w http.ResponseWriter, r *http.Request) {
	var req models.MetricsList
	if err := easyjson.UnmarshalFromReader(r.Body, &req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)

		return
	}

	if err := h.metrics.UpdateMetrics(r.Context(), req); err != nil {
		h.writeServiceError(w, err)

		return
	}

	writePlain(w, http.StatusOK, "metrics saved")
}

func (h *MetricsHandler) valueJSON(w http.ResponseWriter, r *http.Request) {
	var req models.Metrics
	if err := easyjson.UnmarshalFromReader(r.Body, &req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)

		return
	}

	current, err := h.metrics.Metric(r.Context(), req.MType, req.ID)
	if err != nil {
		h.writeServiceError(w, err)

		return
	}

	h.writeJSON(w, http.StatusOK, current)
}

func (h *MetricsHandler) index(w http.ResponseWriter, r *http.Request) {
	all, err := h.metrics.All(r.Context())
	if err != nil {
		h.writeServiceError(w, err)

		return
	}

	views := make([]metricView, 0, len(all))

	for _, m := range all {
		view := metricView{Name: m.ID, Type: m.MType}

		switch {
		case m.MType == models.Gauge && m.Value != nil:
			view.Value = service.FormatGauge(*m.Value)
		case m.MType == models.Counter && m.Delta != nil:
			view.Value = service.FormatCounter(*m.Delta)
		}

		views = append(views, view)
	}

	sort.Slice(views, func(i, j int) bool { return views[i].Name < views[j].Name })

	var buf bytes.Buffer

	if err := indexTemplate.Execute(&buf, views); err != nil {
		h.log.Error("cannot render metrics page", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(buf.Bytes()); err != nil {
		return
	}
}

func (h *MetricsHandler) ping(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
	defer cancel()

	if err := h.pinger.Ping(ctx); err != nil {
		h.log.Warn("storage ping failed", zap.Error(err))
		http.Error(w, "storage is unavailable", http.StatusInternalServerError)

		return
	}

	writePlain(w, http.StatusOK, "OK")
}

func (h *MetricsHandler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrEmptyName), errors.Is(err, service.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, service.ErrUnknownType), errors.Is(err, service.ErrInvalidValue), errors.Is(err, service.ErrEmptyBatch):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		h.log.Error("internal error while handling metrics request", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (h *MetricsHandler) writeJSON(w http.ResponseWriter, status int, m models.Metrics) {
	body, err := easyjson.Marshal(m)
	if err != nil {
		h.log.Error("cannot encode metric", zap.String("metric", m.ID), zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if _, err := w.Write(body); err != nil {
		return
	}
}

func writePlain(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)

	if _, err := w.Write([]byte(body)); err != nil {
		return
	}
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "metric name is not specified", http.StatusNotFound)
}

package handler

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
	"github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/service"
)

type metricView struct {
	Name  string
	Type  string
	Value string
}

type MetricsHandler struct {
	metrics *service.MetricsService
}

func NewMetricsHandler(metrics *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{metrics: metrics}
}

func (h *MetricsHandler) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.StripSlashes)

	r.Get("/", h.index)

	r.Route("/update", func(r chi.Router) {
		r.Post("/{type}/{name}/{value}", h.update)
		r.Post("/", notFound)
		r.Post("/{type}", notFound)
		r.Post("/{type}/{name}", notFound)
	})

	r.Route("/value", func(r chi.Router) {
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

	if err := h.metrics.Update(mType, name, value); err != nil {
		writeServiceError(w, err)

		return
	}

	writePlain(w, http.StatusOK, "metric saved")
}

func (h *MetricsHandler) value(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")

	value, err := h.metrics.Value(mType, name)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writePlain(w, http.StatusOK, value)
}

func (h *MetricsHandler) index(w http.ResponseWriter, _ *http.Request) {
	all := h.metrics.All()

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

	var sb strings.Builder

	for _, view := range views {
		sb.WriteString(view.Name)
		sb.WriteString("\t")
		sb.WriteString(view.Type)
		sb.WriteString("\t")
		sb.WriteString(view.Value)
		sb.WriteString("\n")
	}

	writePlain(w, http.StatusOK, sb.String())
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrEmptyName), errors.Is(err, service.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, service.ErrUnknownType), errors.Is(err, service.ErrInvalidValue):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

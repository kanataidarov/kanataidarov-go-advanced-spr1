package agent

import (
	"testing"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

func snapshotMap(t *testing.T, metrics []models.Metrics) map[string]models.Metrics {
	t.Helper()

	m := make(map[string]models.Metrics, len(metrics))
	for _, metric := range metrics {
		m[metric.ID] = metric
	}

	return m
}

func TestCollectorPollFillsRuntimeGauges(t *testing.T) {
	c := NewCollector()
	c.Poll()

	metrics := snapshotMap(t, c.Snapshot())

	required := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
		"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
		"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys",
		"Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs",
		"StackInuse", "StackSys", "Sys", "TotalAlloc", "RandomValue",
	}

	for _, name := range required {
		metric, ok := metrics[name]
		if !ok {
			t.Fatalf("metric %s is not collected", name)
		}

		if metric.MType != models.Gauge {
			t.Errorf("metric %s: got type %s, want %s", name, metric.MType, models.Gauge)
		}

		if metric.Value == nil {
			t.Errorf("metric %s has nil value", name)
		}
	}
}

func TestCollectorPollCountIncrements(t *testing.T) {
	c := NewCollector()

	for i := int64(1); i <= 3; i++ {
		c.Poll()

		metric, ok := snapshotMap(t, c.Snapshot())[pollCountMetric]
		if !ok {
			t.Fatal("PollCount is not collected")
		}

		if metric.MType != models.Counter {
			t.Fatalf("got type %s, want %s", metric.MType, models.Counter)
		}

		if metric.Delta == nil || *metric.Delta != i {
			t.Fatalf("got PollCount %v, want %d", metric.Delta, i)
		}
	}
}

func TestCollectorSnapshotIsIndependentCopy(t *testing.T) {
	c := NewCollector()
	c.Poll()

	first := snapshotMap(t, c.Snapshot())["Alloc"]
	*first.Value = -1

	second := snapshotMap(t, c.Snapshot())["Alloc"]
	if *second.Value == -1 {
		t.Error("snapshot shares memory with collector state")
	}
}

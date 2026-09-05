package agent

import (
	"math/rand"
	"runtime"
	"sync"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

const (
	pollCountMetric   = "PollCount"
	randomValueMetric = "RandomValue"
)

type Collector struct {
	mu        sync.RWMutex
	gauges    map[string]float64
	pollCount int64
}

func NewCollector() *Collector {
	return &Collector{gauges: make(map[string]float64)}
}

func (c *Collector) Poll() {
	var ms runtime.MemStats

	runtime.ReadMemStats(&ms)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.gauges["Alloc"] = float64(ms.Alloc)
	c.gauges["BuckHashSys"] = float64(ms.BuckHashSys)
	c.gauges["Frees"] = float64(ms.Frees)
	c.gauges["GCCPUFraction"] = ms.GCCPUFraction
	c.gauges["GCSys"] = float64(ms.GCSys)
	c.gauges["HeapAlloc"] = float64(ms.HeapAlloc)
	c.gauges["HeapIdle"] = float64(ms.HeapIdle)
	c.gauges["HeapInuse"] = float64(ms.HeapInuse)
	c.gauges["HeapObjects"] = float64(ms.HeapObjects)
	c.gauges["HeapReleased"] = float64(ms.HeapReleased)
	c.gauges["HeapSys"] = float64(ms.HeapSys)
	c.gauges["LastGC"] = float64(ms.LastGC)
	c.gauges["Lookups"] = float64(ms.Lookups)
	c.gauges["MCacheInuse"] = float64(ms.MCacheInuse)
	c.gauges["MCacheSys"] = float64(ms.MCacheSys)
	c.gauges["MSpanInuse"] = float64(ms.MSpanInuse)
	c.gauges["MSpanSys"] = float64(ms.MSpanSys)
	c.gauges["Mallocs"] = float64(ms.Mallocs)
	c.gauges["NextGC"] = float64(ms.NextGC)
	c.gauges["NumForcedGC"] = float64(ms.NumForcedGC)
	c.gauges["NumGC"] = float64(ms.NumGC)
	c.gauges["OtherSys"] = float64(ms.OtherSys)
	c.gauges["PauseTotalNs"] = float64(ms.PauseTotalNs)
	c.gauges["StackInuse"] = float64(ms.StackInuse)
	c.gauges["StackSys"] = float64(ms.StackSys)
	c.gauges["Sys"] = float64(ms.Sys)
	c.gauges["TotalAlloc"] = float64(ms.TotalAlloc)
	c.gauges[randomValueMetric] = rand.Float64()

	c.pollCount++
}

func (c *Collector) Snapshot() []models.Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := make([]models.Metrics, 0, len(c.gauges)+1)

	for name, value := range c.gauges {
		v := value
		snapshot = append(snapshot, models.Metrics{ID: name, MType: models.Gauge, Value: &v})
	}

	delta := c.pollCount
	snapshot = append(snapshot, models.Metrics{ID: pollCountMetric, MType: models.Counter, Delta: &delta})

	return snapshot
}

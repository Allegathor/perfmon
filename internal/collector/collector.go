package collector

import (
	"math/rand/v2"
	"runtime"
	"time"
)

type Mtcs struct {
	Gauge   map[string]float64
	Counter map[string]int64
}

type Collector struct {
	mtcs         Mtcs
	pollInterval uint
}

func New(pollInterval uint) *Collector {
	m := Mtcs{
		Gauge:   make(map[string]float64),
		Counter: make(map[string]int64),
	}
	return &Collector{
		mtcs:         m,
		pollInterval: pollInterval,
	}
}

func (c *Collector) SetGauge(k string, g float64) {
	c.mtcs.Gauge[k] = g
}

func (c *Collector) SetCounter(k string, counter int64) {
	c.mtcs.Counter[k] = counter
}

func (c *Collector) GaugeStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// total
	c.SetGauge("TotalAlloc", float64(m.TotalAlloc))
	c.SetGauge("Sys", float64(m.Sys))
	c.SetGauge("Lookups", float64(m.Lookups))
	c.SetGauge("Mallocs", float64(m.Mallocs))
	c.SetGauge("Frees", float64(m.Frees))

	c.SetGauge("Alloc", float64(m.Alloc))
	c.SetGauge("BuckHashSys", float64(m.BuckHashSys))

	// heap
	c.SetGauge("HeapAlloc", float64(m.HeapAlloc))
	c.SetGauge("HeapIdle", float64(m.HeapIdle))
	c.SetGauge("HeapInuse", float64(m.HeapInuse))
	c.SetGauge("HeapObjects", float64(m.HeapObjects))
	c.SetGauge("HeapReleased", float64(m.HeapReleased))
	c.SetGauge("HeapSys", float64(m.HeapSys))

	// stack
	c.SetGauge("StackInuse", float64(m.StackInuse))
	c.SetGauge("StackSys", float64(m.StackSys))
	c.SetGauge("MSpanInuse", float64(m.MSpanInuse))
	c.SetGauge("MSpanSys", float64(m.MSpanSys))
	c.SetGauge("MCacheInuse", float64(m.MCacheInuse))
	c.SetGauge("MCacheSys", float64(m.MCacheSys))

	// GC
	c.SetGauge("GCCPUFraction", float64(m.GCCPUFraction))
	c.SetGauge("GCSys", float64(m.GCSys))
	c.SetGauge("LastGC", float64(m.LastGC))
	c.SetGauge("NextGC", float64(m.NextGC))
	c.SetGauge("NumForcedGC", float64(m.NumForcedGC))
	c.SetGauge("NumGC", float64(m.NumGC))
	c.SetGauge("PauseTotalNs", float64(m.PauseTotalNs))

	c.SetGauge("OtherSys", float64(m.OtherSys))
	c.SetGauge("RandomValue", (rand.Float64()*100)+1)
}

func (c *Collector) UpdateCounters() {
	c.SetCounter("PollCount", 1)
}

func (c *Collector) Stats() Mtcs {
	c.GaugeStats()
	c.UpdateCounters()

	return c.mtcs
}

func (c *Collector) Monitor(ch chan Mtcs) {
	for {
		time.Sleep(time.Duration(c.pollInterval) * time.Second)
		go func() {
			stats := c.Stats()
			ch <- stats
		}()
	}
}

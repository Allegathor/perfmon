package collector

import (
	"math/rand/v2"
	"runtime"
	"sync"
	"time"

	"github.com/Allegathor/perfmon/internal/mondata"
)

type MtcsTx[T mondata.VTypes] struct {
	repo     *Mtcs[T]
	writable bool
}

func (tx *MtcsTx[T]) Lock() {
	if tx.writable {
		tx.repo.mu.Lock()
	} else {
		tx.repo.mu.RLock()
	}
}

func (tx *MtcsTx[T]) Unlock() {
	if tx.writable {
		tx.repo.mu.Unlock()
	} else {
		tx.repo.mu.RUnlock()
	}
}

func (tx *MtcsTx[T]) Get(name string) (T, bool) {
	v, ok := tx.repo.Data[name]
	return v, ok
}

func (tx *MtcsTx[T]) GetAll() map[string]T {
	return tx.repo.Data
}

func (tx *MtcsTx[T]) Set(name string, v T) {
	tx.repo.Data[name] = v
}

type Mtcs[T mondata.VTypes] struct {
	mu   sync.RWMutex
	Data map[string]T
}

func (r *Mtcs[T]) Begin(writable bool) (*MtcsTx[T], error) {
	tx := &MtcsTx[T]{
		repo:     r,
		writable: writable,
	}
	tx.Lock()

	return tx, nil
}

func (r *Mtcs[T]) managed(writable bool, fn func(*MtcsTx[T]) error) (err error) {
	tx, err := r.Begin(writable)
	if err != nil {
		return err
	}

	defer func() {
		tx.Unlock()
	}()

	err = fn(tx)
	return nil
}

func (r *Mtcs[T]) Read(fn func(*MtcsTx[T]) error) error {
	return r.managed(false, fn)
}

func (r *Mtcs[T]) Update(fn func(*MtcsTx[T]) error) error {
	return r.managed(true, fn)
}

type Repo struct {
	Gauge   *Mtcs[float64]
	Counter *Mtcs[int64]
}

type Collector struct {
	repo         *Repo
	pollInterval uint
}

func New(pollInterval uint) *Collector {
	g := &Mtcs[float64]{
		Data: make(map[string]float64),
	}

	c := &Mtcs[int64]{
		Data: make(map[string]int64),
	}

	return &Collector{
		repo: &Repo{
			Gauge:   g,
			Counter: c,
		},
		pollInterval: pollInterval,
	}
}

func (c *Collector) GaugeStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	go func() {
		c.repo.Gauge.Update(func(tx *MtcsTx[float64]) error {
			// total
			tx.Set("TotalAlloc", float64(m.TotalAlloc))
			tx.Set("Sys", float64(m.Sys))
			tx.Set("Lookups", float64(m.Lookups))
			tx.Set("Mallocs", float64(m.Mallocs))
			tx.Set("Frees", float64(m.Frees))

			tx.Set("Alloc", float64(m.Alloc))
			tx.Set("BuckHashSys", float64(m.BuckHashSys))

			// heap
			tx.Set("HeapAlloc", float64(m.HeapAlloc))
			tx.Set("HeapIdle", float64(m.HeapIdle))
			tx.Set("HeapInuse", float64(m.HeapInuse))
			tx.Set("HeapObjects", float64(m.HeapObjects))
			tx.Set("HeapReleased", float64(m.HeapReleased))
			tx.Set("HeapSys", float64(m.HeapSys))

			// stack
			tx.Set("StackInuse", float64(m.StackInuse))
			tx.Set("StackSys", float64(m.StackSys))
			tx.Set("MSpanInuse", float64(m.MSpanInuse))
			tx.Set("MSpanSys", float64(m.MSpanSys))
			tx.Set("MCacheInuse", float64(m.MCacheInuse))
			tx.Set("MCacheSys", float64(m.MCacheSys))

			// GC
			tx.Set("GCCPUFraction", float64(m.GCCPUFraction))
			tx.Set("GCSys", float64(m.GCSys))
			tx.Set("LastGC", float64(m.LastGC))
			tx.Set("NextGC", float64(m.NextGC))
			tx.Set("NumForcedGC", float64(m.NumForcedGC))
			tx.Set("NumGC", float64(m.NumGC))
			tx.Set("PauseTotalNs", float64(m.PauseTotalNs))

			tx.Set("OtherSys", float64(m.OtherSys))
			tx.Set("RandomValue", (rand.Float64()*100)+1)
			return nil
		})
	}()
}

func (c *Collector) UpdateCounters() {

	go func() {
		c.repo.Counter.Update(func(tx *MtcsTx[int64]) error {
			tx.Set("PollCount", 1)

			return nil
		})
	}()
}

func (c *Collector) Stats() Repo {
	c.GaugeStats()
	c.UpdateCounters()

	return *c.repo
}

func (c *Collector) Monitor(ch chan Repo) {
	for {
		time.Sleep(time.Duration(c.pollInterval) * time.Second)
		go func() {
			stats := c.Stats()
			ch <- stats
		}()
	}
}

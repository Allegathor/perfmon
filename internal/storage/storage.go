package storage

import (
	"sync"
	"time"
)

type Storage interface {
	SetGauge(name string, v float64)
	UpdateCounter(name string, v int64)
}

type MetricsStorage struct {
	WG      sync.WaitGroup
	Gauge   map[string]float64
	Counter map[string]int64
}

func NewMetrics() *MetricsStorage {
	return &MetricsStorage{
		WG:      sync.WaitGroup{},
		Gauge:   make(map[string]float64),
		Counter: make(map[string]int64),
	}
}

func (s *MetricsStorage) SetGauge(name string, v float64) {
	time.Sleep(time.Second)
	go func() {
		defer s.WG.Done()
		s.Gauge[name] = v
	}()
}

func (s *MetricsStorage) SetCounter(name string, v int64) {
	time.Sleep(time.Second)
	go func() {
		defer s.WG.Done()
		if _, ok := s.Counter[name]; ok {
			s.Counter[name] += v
		}
		s.Counter[name] = v
	}()
}

package storage

type Storage interface {
	SetGauge(name string, v float64)
	UpdateCounter(name string, v int64)
}

type MetricsStorage struct {
	Gauge   map[string]float64
	Counter map[string]int64
}

func NewMetrics() *MetricsStorage {
	return &MetricsStorage{
		Gauge:   make(map[string]float64),
		Counter: make(map[string]int64),
	}
}

func (s *MetricsStorage) SetGauge(name string, v float64) {
	s.Gauge[name] = v
}

func (s *MetricsStorage) SetCounter(name string, v int64) {
	if _, ok := s.Counter[name]; ok {
		s.Counter[name] += v
	}
	s.Counter[name] = v
}

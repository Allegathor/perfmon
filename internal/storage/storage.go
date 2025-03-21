package storage

type GaugeMap = map[string]float64
type CounterMap = map[string]int64

type MetricsStorage struct {
	Gauge   GaugeMap
	Counter CounterMap
}

func NewMetrics() *MetricsStorage {
	return &MetricsStorage{
		Gauge:   make(GaugeMap),
		Counter: make(CounterMap),
	}
}

func (s *MetricsStorage) GetGauge(name string) (float64, bool) {
	v, ok := s.Gauge[name]
	return v, ok
}

func (s *MetricsStorage) GetGaugeAll() GaugeMap {
	return s.Gauge
}

func (s *MetricsStorage) SetGauge(name string, v float64) {
	s.Gauge[name] = v
}

func (s *MetricsStorage) GetCounter(name string) (int64, bool) {
	v, ok := s.Counter[name]
	return v, ok
}

func (s *MetricsStorage) GetCounterAll() CounterMap {
	return s.Counter
}

func (s *MetricsStorage) SetCounter(name string, v int64) {
	if _, ok := s.Counter[name]; ok {
		s.Counter[name] += v
		return
	}
	s.Counter[name] = v
}

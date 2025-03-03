package storage

type Storage interface {
	Add(rec MetricRec)
	GetHistory() []MetricRec
}

type MetricRec struct {
	ValueType  string
	Name       string
	GaugeVal   float64
	CounterVal int64
}

type MetricsStorage struct {
	list []MetricRec
}

func NewMetrics() *MetricsStorage {
	return &MetricsStorage{}
}

func (s *MetricsStorage) Add(rec MetricRec) {
	s.list = append(s.list, rec)
}

func (s *MetricsStorage) GetHistory() []MetricRec {
	return s.list
}

package mondata

import (
	"strconv"
	"strings"
)

type Metrics struct {
	ID     string   `json:"id"`
	MType  string   `json:"type"`
	Delta  *int64   `json:"delta,omitempty"`
	Value  *float64 `json:"value,omitempty"`
	PValue string   `json:"-"`
}

const (
	GaugeType   = "gauge"
	CounterType = "counter"
)

type Gauge struct {
	MetricType string
	Name       string
	Value      float64
}

type GaugeValue interface {
	int | int32 | int64 | uint | uint32 | uint64 | float32 | float64
}

func ParseGauge(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func FormatGauge(f float64) string {
	return strings.TrimRight(strconv.FormatFloat(f, 'f', -1, 64), "0.")
}

func NewGauge[V GaugeValue](name string, f V) *Gauge {
	return &Gauge{
		MetricType: GaugeType,
		Name:       name,
		Value:      float64(f),
	}
}

func (g *Gauge) String() string {
	return FormatGauge(g.Value)
}

type Counter struct {
	MetricType string
	Name       string
	Value      int64
}

func ParseCounter(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func FormatCounter(f int64) string {
	return strconv.FormatInt(f, 10)
}

func NewCounter(name string, i int64) *Counter {
	return &Counter{
		MetricType: CounterType,
		Name:       name,
		Value:      i,
	}
}

func (g *Counter) String() string {
	return FormatCounter(g.Value)
}

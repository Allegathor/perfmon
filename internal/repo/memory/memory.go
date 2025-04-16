package memory

import (
	"context"
	"errors"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo/safe"
	"github.com/Allegathor/perfmon/internal/repo/transaction"
	"go.uber.org/zap"
)

type MemorySt struct {
	Gauge   *safe.MRepo[mondata.GaugeVType]
	Counter *safe.MRepo[mondata.CounterVType]
	logger  *zap.SugaredLogger
}

func Init(ctx context.Context, logger *zap.SugaredLogger) (*MemorySt, error) {
	return &MemorySt{
		Gauge:   safe.NewMRepo[mondata.GaugeVType](),
		Counter: safe.NewMRepo[mondata.CounterVType](),
		logger:  logger,
	}, nil
}

// for tests
func InitEmpty() *MemorySt {
	return &MemorySt{
		Gauge:   safe.NewMRepo[mondata.GaugeVType](),
		Counter: safe.NewMRepo[mondata.CounterVType](),
		logger:  nil,
	}
}

func (ms *MemorySt) Close() {
	// TODO
}

func (ms *MemorySt) log(args ...any) {
	if ms.logger != nil {
		ms.logger.Infoln(args...)
	}
}

// MARK: gauge metrics
func (ms *MemorySt) GetGauge(ctx context.Context, name string) (mondata.GaugeVType, bool, error) {
	var (
		v  mondata.GaugeVType
		ok = false
	)

	ms.Gauge.Read(func(tx transaction.TxQry[mondata.GaugeVType]) error {
		v, ok = tx.Get(name)
		return nil
	})

	ms.log("read gauge value from memstorage", "name:", name, "ok:", ok, "value:", v)
	return v, ok, nil
}

func (ms *MemorySt) GetGaugeAll(ctx context.Context) (mondata.GaugeMap, error) {
	var m mondata.GaugeMap

	ms.Gauge.Read(func(tx transaction.TxQry[mondata.GaugeVType]) error {
		m = tx.GetAll()
		return nil
	})

	ms.log("read all gauge values from memstorage, values:", m)
	return m, nil
}

func (ms *MemorySt) SetGauge(ctx context.Context, name string, value mondata.GaugeVType) error {
	ms.Gauge.Update(func(tx transaction.TxExec[mondata.GaugeVType]) error {
		tx.Set(name, value)
		return nil
	})

	ms.log("set gauge value in memstorage", "name:", name, "value:", value)
	return nil
}

func (ms *MemorySt) SetGaugeAll(ctx context.Context, metrics mondata.GaugeMap) error {
	ms.Gauge.Update(func(tx transaction.TxExec[mondata.GaugeVType]) error {
		tx.SetAll(metrics)
		return nil
	})

	ms.log("set all gauge values in memstorage, values:", metrics)
	return nil
}

// MARK: counter metrics
func (ms *MemorySt) GetCounter(ctx context.Context, name string) (mondata.CounterVType, bool, error) {
	var (
		v  mondata.CounterVType
		ok = false
	)

	ms.Counter.Read(func(tx transaction.TxQry[mondata.CounterVType]) error {
		v, ok = tx.Get(name)
		return nil
	})

	ms.log("read counter value from memstorage", "name:", name, "ok:", ok, "value:", v)
	return v, ok, nil
}

func (ms *MemorySt) GetCounterAll(ctx context.Context) (mondata.CounterMap, error) {
	var m mondata.CounterMap
	ms.Counter.Read(func(tx transaction.TxQry[mondata.CounterVType]) error {
		m = tx.GetAll()
		return nil
	})

	ms.log("read all counter values from memstorage, values", m)
	return m, nil
}

func (ms *MemorySt) SetCounter(ctx context.Context, name string, value mondata.CounterVType) error {
	ms.Counter.Update(func(tx transaction.TxExec[mondata.CounterVType]) error {
		tx.SetAccum(name, value)
		return nil
	})

	ms.log("set counter value in memstorage", "name:", name, "value:", value)
	return nil
}

func (ms *MemorySt) SetCounterAll(ctx context.Context, values map[string]mondata.CounterVType) error {
	ms.Counter.Update(func(tx transaction.TxExec[mondata.CounterVType]) error {
		tx.SetAccumAll(values)
		return nil
	})

	ms.log("set all counter values in memstorage", values)
	return nil
}

func (ms *MemorySt) Ping(ctx context.Context) error {
	return errors.New("there is no connection to remote db, in-memory storage is used")
}

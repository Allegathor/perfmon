package repo

import (
	"context"
	"fmt"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo/memory"
	"github.com/Allegathor/perfmon/internal/repo/pgsql"
)

type MetricsGetters interface {
	GetGauge(ctx context.Context, name string) (mondata.GaugeVType, bool, error)
	GetGaugeAll(ctx context.Context) (mondata.GaugeMap, error)

	GetCounter(ctx context.Context, name string) (mondata.CounterVType, bool, error)
	GetCounterAll(ctx context.Context) (mondata.CounterMap, error)
}

type MetricsSetters interface {
	SetGauge(ctx context.Context, name string, value mondata.GaugeVType) error
	SetGaugeAll(ctx context.Context, gaugeMap mondata.GaugeMap) error

	SetCounter(ctx context.Context, name string, value mondata.CounterVType) error
	SetCounterAll(ctx context.Context, gaugeMap mondata.CounterMap) error
}

type MetricsRepo interface {
	MetricsGetters
	MetricsSetters
	Ping(ctx context.Context) error
}

type backupWriter interface {
	RestorePrev(MetricsRepo) error
	ShouldRestore() bool
	Write(MetricsRepo) error
}

type Current struct {
	MetricsRepo
	bkp backupWriter
}

func (c *Current) Dump() {
	c.bkp.Write(c.MetricsRepo)
}

func Init(ctx context.Context, connStr string, bkp backupWriter) *Current {

	if connStr != "" {
		if pg, err := pgsql.Init(ctx, connStr); err != nil {
			fmt.Println(err.Error())
		} else {
			return &Current{MetricsRepo: pg}
		}
	}

	ms, _ := memory.Init(ctx)
	if bkp.ShouldRestore() {
		bkp.RestorePrev(ms)
	}

	return &Current{MetricsRepo: ms, bkp: bkp}
}

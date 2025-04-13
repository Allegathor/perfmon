package repo

import (
	"context"
	"fmt"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/Allegathor/perfmon/internal/repo/memory"
	"github.com/Allegathor/perfmon/internal/repo/pgsql"
	"go.uber.org/zap"
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
	Close()
}

type backupWriter interface {
	RestorePrev(MetricsRepo) error
	ShouldRestore() bool
	Schedule(context.Context, MetricsRepo) error
}

type Current struct {
	MetricsRepo
	bkp        backupWriter
	logger     *zap.SugaredLogger
	isInMemory bool
}

func Init(ctx context.Context, connStr string, bkp backupWriter, logger *zap.SugaredLogger) *Current {

	if connStr != "" {
		if pg, err := pgsql.Init(ctx, connStr); err != nil {
			fmt.Println(err.Error())
		} else {
			return &Current{MetricsRepo: pg, bkp: bkp, isInMemory: false}
		}
	}

	l := logger.Named("repo")
	ms, _ := memory.Init(ctx, l)

	return &Current{MetricsRepo: ms, bkp: bkp, logger: l, isInMemory: true}
}

func (c *Current) Restore() error {
	if c.bkp.ShouldRestore() {
		err := c.bkp.RestorePrev(c.MetricsRepo)
		if err != nil {
			c.logger.Error("values couldn't be restored from backup, error: ", err)
			return err
		}
		c.logger.Info("values were restored from backup file with success")
		return nil
	}

	c.logger.Warn("restore flag wasn't set")
	return nil
}

func (c *Current) ScheduleBackup(ctx context.Context) error {
	if c.isInMemory {
		return c.bkp.Schedule(ctx, c.MetricsRepo)
	}

	return nil
}

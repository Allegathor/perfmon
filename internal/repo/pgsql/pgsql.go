package pgsql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var createGaugeQry = `
	CREATE TABLE IF NOT EXISTS gauge_m_table (
		m_id SERIAL PRIMARY KEY,
		name VARCHAR(64) UNIQUE,
		value DOUBLE PRECISION NOT NULL DEFAULT 0
	);
`

var createCounterQry = `
	CREATE TABLE IF NOT EXISTS counter_m_table (
		m_id SERIAL PRIMARY KEY,
		name VARCHAR(64) UNIQUE,
		value INTEGER NOT NULL DEFAULT 0
	);
`

type PgSQL struct {
	*pgxpool.Pool
	logger *zap.SugaredLogger
}

func (pg *PgSQL) Close() {
	pg.Pool.Close()
}

func IsRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if err == nil {
		return false
	}

	if err == context.DeadlineExceeded || err == context.Canceled {
		return true
	}

	if !errors.As(err, &pgErr) {
		return false
	}

	switch pgErr.Code {
	case
		pgerrcode.SerializationFailure,
		pgerrcode.DeadlockDetected:
		return true
	}

	return pgerrcode.IsConnectionException(pgErr.Code)
}

const maxRetryes = 3

func (pg *PgSQL) ExecuteTx(
	ctx context.Context, txOptions pgx.TxOptions, fnc func(pgx.Tx) error,
) error {
	retry := retrypolicy.Builder[any]().HandleIf(func(_ any, err error) bool {
		return IsRetryable(err)
	}).
		WithMaxRetries(maxRetryes).
		WithDelayFunc(func(exec failsafe.ExecutionAttempt[any]) time.Duration {
			return time.Second + time.Duration(exec.Attempts()-1)*2*time.Second
		}).
		Build()

	return failsafe.NewExecutor(retry).
		WithContext(ctx).
		RunWithExecution(func(exec failsafe.Execution[any]) (err error) {
			ctx := exec.Context()
			tx, err := pg.BeginTx(ctx, txOptions)
			if err != nil {
				return fmt.Errorf("failed to begin tx: %w", err)
			}

			defer tx.Rollback(ctx)

			if err := fnc(tx); err != nil {
				return err
			}

			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("failed to commit: %w", err)
			}

			return err
		})
}

func Init(ctx context.Context, connStr string, logger *zap.SugaredLogger) (*PgSQL, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}

	config.MaxConns = 15
	config.MinConns = 2
	config.MaxConnIdleTime = 20 * time.Second
	config.HealthCheckPeriod = 10 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	pg := &PgSQL{Pool: pool, logger: logger}

	err = pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadWrite},
		func(tx pgx.Tx) error {
			_, err = tx.Exec(ctx, createGaugeQry)
			if err != nil {
				return err
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadWrite},
		func(tx pgx.Tx) error {
			_, err = tx.Exec(ctx, createCounterQry)
			if err != nil {
				return err
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	return pg, nil
}

// MARK: gauge metrics
func (pg *PgSQL) GetGauge(ctx context.Context, name string) (mondata.GaugeVType, bool, error) {
	var v mondata.GaugeVType

	err := pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadWrite},
		func(tx pgx.Tx) error {
			row := tx.QueryRow(ctx, `
			SELECT value FROM gauge_m_table WHERE name = @name
		`, pgx.NamedArgs{"name": name})

			if err := row.Scan(&v); err != nil {
				return err
			}

			return nil
		})
	if err != nil {
		return 0, false, err
	}

	return v, true, nil
}

func (pg *PgSQL) GetGaugeAll(ctx context.Context) (mondata.GaugeMap, error) {
	gm := make(mondata.GaugeMap)

	err := pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadWrite},
		func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
			SELECT name, value FROM gauge_m_table
		`)
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var (
					k string
					v mondata.GaugeVType
				)

				if err = rows.Scan(&k, &v); err != nil {
					return err
				}

				gm[k] = v
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	return gm, nil
}

var upsertGaugeQry = `
	INSERT INTO gauge_m_table (name, value)
	VALUES (@name, @value)
	ON CONFLICT(name)
	DO UPDATE SET
		value = @value;
`

func (pg *PgSQL) SetGauge(ctx context.Context, name string, value mondata.GaugeVType) error {
	return pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadWrite},
		func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, upsertGaugeQry, pgx.NamedArgs{"name": name, "value": value})
			if err != nil {
				return err
			}

			return nil
		})
}

func (pg *PgSQL) SetGaugeAll(ctx context.Context, metrics mondata.GaugeMap) error {
	return pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadWrite},
		func(tx pgx.Tx) error {
			for k, v := range metrics {
				_, err := tx.Exec(ctx, upsertGaugeQry, pgx.NamedArgs{"name": k, "value": v})
				if err != nil {
					return err
				}
			}

			return nil
		})
}

// MARK: counter metrics
func (pg *PgSQL) GetCounter(ctx context.Context, name string) (mondata.CounterVType, bool, error) {
	var v mondata.CounterVType

	err := pg.ExecuteTx(
		ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly},
		func(tx pgx.Tx) error {

			row := tx.QueryRow(ctx, `
			SELECT value FROM counter_m_table WHERE name = @name
		`, pgx.NamedArgs{"name": name})

			if err := row.Scan(&v); err != nil {
				return err
			}

			return nil
		})
	if err != nil {
		return 0, false, err
	}

	return v, true, nil
}

func (pg *PgSQL) GetCounterAll(ctx context.Context) (mondata.CounterMap, error) {
	cm := make(mondata.CounterMap)

	err := pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadOnly},
		func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
			SELECT name, value FROM counter_m_table
		`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var (
					k string
					v mondata.CounterVType
				)

				if err = rows.Scan(&k, &v); err != nil {
					return err
				}

				cm[k] = v
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	return cm, nil
}

var upsertCounterQry = `
	INSERT INTO counter_m_table (name, value)
	VALUES (@name, @value)
	ON CONFLICT(name)
	DO UPDATE SET
		value = counter_m_table.value + @value;
`

func (pg *PgSQL) SetCounter(ctx context.Context, name string, value mondata.CounterVType) error {
	return pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadWrite},
		func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, upsertCounterQry, pgx.NamedArgs{"name": name, "value": value})
			if err != nil {
				return err
			}

			return nil
		})
}

func (pg *PgSQL) SetCounterAll(ctx context.Context, metrics mondata.CounterMap) error {
	return pg.ExecuteTx(
		ctx,
		pgx.TxOptions{AccessMode: pgx.ReadWrite},
		func(tx pgx.Tx) error {
			for k, v := range metrics {
				_, err := tx.Exec(ctx, upsertCounterQry, pgx.NamedArgs{"name": k, "value": v})
				if err != nil {
					return err
				}
			}

			return nil
		})
}

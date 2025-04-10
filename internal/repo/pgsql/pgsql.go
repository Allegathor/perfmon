package pgsql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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
}

func Init(ctx context.Context, connStr string) (*PgSQL, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, err
	}

	config.MaxConns = 15
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 2 * time.Minute
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, createGaugeQry)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, createCounterQry)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return &PgSQL{Pool: pool}, nil
}

func (pg *PgSQL) Close() {
	pg.Pool.Close()
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if err == context.DeadlineExceeded || err == context.Canceled {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.AdminShutdown,
			pgerrcode.CrashShutdown,
			pgerrcode.CannotConnectNow,
			pgerrcode.ConnectionException,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure,
			pgerrcode.SQLClientUnableToEstablishSQLConnection,
			pgerrcode.SQLServerRejectedEstablishmentOfSQLConnection,
			pgerrcode.TransactionResolutionUnknown,
			pgerrcode.SerializationFailure,
			pgerrcode.DeadlockDetected:
			return true
		}
	}

	return true
}

func Retry(ctx context.Context, retryCount int, initialBackoff time.Duration, fn func() error) error {
	var err error
	backoff := initialBackoff

	for i := range retryCount {
		err = fn()
		if err == nil {
			return nil
		}

		if !IsRetryable(err) {
			return err
		}

		if i < retryCount-1 {
			select {
			case <-time.After(backoff):
				backoff += 2
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("after %d retries: %w", retryCount, err)
}

func (pg *PgSQL) QueryRowWithRetry(ctx context.Context, query string, args ...any) (pgx.Row, error) {
	var row pgx.Row

	retryErr := Retry(ctx, 3, time.Second, func() error {
		row = pg.QueryRow(ctx, query, args...)
		return nil
	})

	if retryErr != nil {
		return nil, retryErr
	}

	return row, nil
}

func (pg *PgSQL) QueryWithRetry(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	var rows pgx.Rows
	var err error

	retryErr := Retry(ctx, 3, time.Second, func() error {
		rows, err = pg.Query(ctx, query, args...)
		return err
	})

	if retryErr != nil {
		return nil, retryErr
	}

	return rows, nil
}

func (pg *PgSQL) ExecWithRetry(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	var result pgconn.CommandTag
	var err error

	retryErr := Retry(ctx, 3, time.Second, func() error {
		result, err = pg.Exec(ctx, sql, args...)
		return err
	})

	if retryErr != nil {
		return pgconn.NewCommandTag(""), retryErr
	}

	return result, nil
}

func (pg *PgSQL) ExecuteInTransaction(ctx context.Context, retryCount int, fn func(pgx.Tx) error) error {
	backoff := time.Second

	for i := range retryCount {
		// Begin transaction
		tx, err := pg.Begin(ctx)
		if err != nil {
			if IsRetryable(err) && i < retryCount-1 {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		err = fn(tx)
		if err == nil {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				if IsRetryable(commitErr) && i < retryCount-1 {
					time.Sleep(backoff)
					backoff += 2
					continue
				}
				return fmt.Errorf("failed to commit transaction: %w", commitErr)
			}
			return nil
		}

		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			fmt.Printf("warning: failed to rollback transaction: %v", rollbackErr)
		}

		if !IsRetryable(err) || i >= retryCount-1 {
			return err
		}

		select {
		case <-time.After(backoff):
			backoff += 2
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("max retries (%d) reached", retryCount)
}

// MARK: gauge metrics
func (pg *PgSQL) GetGauge(ctx context.Context, name string) (mondata.GaugeVType, bool, error) {
	var v mondata.GaugeVType

	err := pg.ExecuteInTransaction(ctx, 3, func(tx pgx.Tx) error {
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

	err := pg.ExecuteInTransaction(ctx, 3, func(tx pgx.Tx) error {
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
	return pg.ExecuteInTransaction(ctx, 3, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, upsertGaugeQry, pgx.NamedArgs{"name": name, "value": value})
		if err != nil {
			return err
		}

		return nil
	})
}

func (pg *PgSQL) SetGaugeAll(ctx context.Context, metrics mondata.GaugeMap) error {
	return pg.ExecuteInTransaction(ctx, 3, func(tx pgx.Tx) error {
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

	err := pg.ExecuteInTransaction(ctx, 3, func(tx pgx.Tx) error {
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

	err := pg.ExecuteInTransaction(ctx, 3, func(tx pgx.Tx) error {
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
	return pg.ExecuteInTransaction(ctx, 3, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, upsertCounterQry, pgx.NamedArgs{"name": name, "value": value})
		if err != nil {
			return err
		}

		return nil
	})
}

func (pg *PgSQL) SetCounterAll(ctx context.Context, metrics mondata.CounterMap) error {
	return pg.ExecuteInTransaction(ctx, 3, func(tx pgx.Tx) error {
		for k, v := range metrics {
			_, err := tx.Exec(ctx, upsertCounterQry, pgx.NamedArgs{"name": k, "value": v})
			if err != nil {
				return err
			}
		}

		return nil
	})
}

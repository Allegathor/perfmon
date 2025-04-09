package pgsql

import (
	"context"

	"github.com/Allegathor/perfmon/internal/mondata"
	"github.com/jackc/pgx/v5"
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
	conn, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
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

	return &PgSQL{Pool: conn}, nil
}

// MARK: gauge metrics
func (pg *PgSQL) GetGauge(ctx context.Context, name string) (mondata.GaugeVType, bool, error) {
	var (
		tx  pgx.Tx
		err error
	)

	if tx, err = pg.Begin(ctx); err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT value FROM gauge_m_table WHERE name = @name
	`, pgx.NamedArgs{"name": name})

	var v mondata.GaugeVType
	if err = row.Scan(&v); err != nil {
		return 0, false, err
	}

	return v, true, tx.Commit(ctx)
}

func (pg *PgSQL) GetGaugeAll(ctx context.Context) (mondata.GaugeMap, error) {
	var (
		tx  pgx.Tx
		err error
	)
	gm := make(mondata.GaugeMap)

	if tx, err = pg.Begin(ctx); err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT name, value FROM gauge_m_table
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			k string
			v mondata.GaugeVType
		)

		if err = rows.Scan(&k, &v); err != nil {
			return nil, err
		}

		gm[k] = v
	}

	return gm, tx.Commit(ctx)
}

var upsertGaugeQry = `
	INSERT INTO gauge_m_table (name, value)
	VALUES (@name, @value)
	ON CONFLICT(name)
	DO UPDATE SET
		value = @value;
`

func (pg *PgSQL) SetGauge(ctx context.Context, name string, value mondata.GaugeVType) error {
	var (
		tx  pgx.Tx
		err error
	)

	if tx, err = pg.Begin(ctx); err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, upsertGaugeQry, pgx.NamedArgs{"name": name, "value": value})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (pg *PgSQL) SetGaugeAll(ctx context.Context, metrics mondata.GaugeMap) error {
	var (
		tx  pgx.Tx
		err error
	)

	if tx, err = pg.Begin(ctx); err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Prepare(ctx, "set_g_all", upsertGaugeQry)
	if err != nil {
		return err
	}

	for k, v := range metrics {
		_, err = tx.Exec(ctx, "set_g_all", pgx.NamedArgs{"name": k, "value": v})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// MARK: counter metrics
func (pg *PgSQL) GetCounter(ctx context.Context, name string) (mondata.CounterVType, bool, error) {
	var (
		tx  pgx.Tx
		err error
	)

	if tx, err = pg.Begin(ctx); err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT value FROM counter_m_table WHERE name = @name
	`, pgx.NamedArgs{"name": name})

	var v mondata.CounterVType
	if err = row.Scan(&v); err != nil {
		return 0, false, err
	}

	return v, true, tx.Commit(ctx)
}

func (pg *PgSQL) GetCounterAll(ctx context.Context) (mondata.CounterMap, error) {
	var (
		tx  pgx.Tx
		err error
	)
	cm := make(mondata.CounterMap)

	if tx, err = pg.Begin(ctx); err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT name, value FROM counter_m_table
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			k string
			v mondata.CounterVType
		)

		if err = rows.Scan(&k, &v); err != nil {
			return nil, err
		}

		cm[k] = v
	}

	return cm, tx.Commit(ctx)
}

var upsertCounterQry = `
	INSERT INTO counter_m_table (name, value)
	VALUES (@name, @value)
	ON CONFLICT(name)
	DO UPDATE SET
		value = counter_m_table.value + @value;
`

func (pg *PgSQL) SetCounter(ctx context.Context, name string, value mondata.CounterVType) error {
	var (
		tx  pgx.Tx
		err error
	)

	if tx, err = pg.Begin(ctx); err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, upsertCounterQry, pgx.NamedArgs{"name": name, "value": value})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (pg *PgSQL) SetCounterAll(ctx context.Context, metrics mondata.CounterMap) error {
	var (
		tx  pgx.Tx
		err error
	)

	if tx, err = pg.Begin(ctx); err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Prepare(ctx, "set_c_all", upsertCounterQry)
	if err != nil {
		return err
	}

	for k, v := range metrics {
		_, err = tx.Exec(ctx, "set_c_all", pgx.NamedArgs{"name": k, "value": v})
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

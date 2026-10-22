package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// scoped runs each statement in a transaction that sets search_path for
// itself alone, so the CRM finds its own schema on a pool whose connections
// belong to its host and keep the host's settings.
type scoped struct {
	pool  *pgxpool.Pool
	begin pgx.TxOptions
}

func newScoped(pool *pgxpool.Pool, schema string) scoped {
	return scoped{pool: pool, begin: pgx.TxOptions{BeginQuery: "BEGIN; SET LOCAL search_path TO " + searchPath(schema)}}
}

func searchPath(schema string) string {
	return pgx.Identifier{schema}.Sanitize() + ", public"
}

func (s scoped) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.BeginTx(ctx, s.begin)
}

func (s scoped) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	err := pgx.BeginFunc(ctx, s, func(tx pgx.Tx) error {
		var err error
		tag, err = tx.Exec(ctx, sql, args...)
		return err
	})
	return tag, err
}

func (s scoped) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return &scopedRows{Rows: rows, ctx: ctx, tx: tx}, nil
}

func (s scoped) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx, err := s.Begin(ctx)
	if err != nil {
		return failedRow{err}
	}
	return scopedRow{row: tx.QueryRow(ctx, sql, args...), ctx: ctx, tx: tx}
}

// scopedRows ends its transaction once the rows are read or closed. A failed
// statement leaves the transaction aborted, so its commit rolls back and the
// statement's own error is the one reported.
type scopedRows struct {
	pgx.Rows
	ctx context.Context
	tx  pgx.Tx
	err error
}

func (r *scopedRows) Next() bool {
	if r.Rows.Next() {
		return true
	}
	r.Close()
	return false
}

func (r *scopedRows) Close() {
	r.Rows.Close()
	if r.tx != nil {
		r.err = r.tx.Commit(r.ctx)
		r.tx = nil
	}
}

func (r *scopedRows) Err() error {
	if err := r.Rows.Err(); err != nil {
		return err
	}
	return r.err
}

type scopedRow struct {
	row pgx.Row
	ctx context.Context
	tx  pgx.Tx
}

func (r scopedRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	if commit := r.tx.Commit(r.ctx); err == nil {
		err = commit
	}
	return err
}

type failedRow struct {
	err error
}

func (r failedRow) Scan(...any) error {
	return r.err
}

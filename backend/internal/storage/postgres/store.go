package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	authdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/auth"
	conndb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/connections"
	intdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/interactions"
	logodb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/logos"
	recdb "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/generated/records"
	datamigrations "github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

// beginner starts transactions: the pool, or a transaction, in which Begin
// opens a savepoint.
type beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Store struct {
	pool *pgxpool.Pool
	db   beginner
	auth *authdb.Queries
	rec  *recdb.Queries
	conn *conndb.Queries
	in   *intdb.Queries
	logo *logodb.Queries
}

// Open connects and migrates to the latest schema.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, db: pool, auth: authdb.New(pool), rec: recdb.New(pool), conn: conndb.New(pool), in: intdb.New(pool), logo: logodb.New(pool)}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	// The server and worker start together; a session lock runs one's
	// migrations at a time.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, dir, goose.WithDisableGlobalRegistry(true), goose.WithSessionLocker(locker), goose.WithGoMigrations(datamigrations.DealFollowups, datamigrations.FollowUps, datamigrations.ChaseFilter))
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// tx runs fn with the auth and records queries bound to one transaction.
func (s *Store) tx(ctx context.Context, fn func(a *authdb.Queries, r *recdb.Queries) error) error {
	return mapError(pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		return fn(s.auth.WithTx(tx), s.rec.WithTx(tx))
	}))
}

// Atomically runs fn against a copy of the store bound to one transaction;
// the store's own transactions nest in it as savepoints.
func (s *Store) Atomically(ctx context.Context, fn func(storage.InteractionStore) error) error {
	return mapError(pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		return fn(&Store{pool: s.pool, db: tx, auth: s.auth.WithTx(tx), rec: s.rec.WithTx(tx), conn: s.conn.WithTx(tx), in: s.in.WithTx(tx), logo: s.logo.WithTx(tx)})
	}))
}

func one[R, T any](conv func(R) T) func(R, error) (T, error) {
	return func(row R, err error) (T, error) {
		if err != nil {
			var zero T
			return zero, mapError(err)
		}
		return conv(row), nil
	}
}

func many[R, T any](conv func(R) T) func([]R, error) ([]T, error) {
	return func(rows []R, err error) ([]T, error) {
		if err != nil {
			return nil, mapError(err)
		}
		out := make([]T, len(rows))
		for i, row := range rows {
			out[i] = conv(row)
		}
		return out, nil
	}
}

func affected(n int64, err error) error {
	if err != nil {
		return mapError(err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return storage.ErrNotFound
	}
	var pgErr *pgconn.PgError
	switch {
	case !errors.As(err, &pgErr):
		return err
	case pgErr.Code == "22P02":
		return storage.ErrNotFound
	case pgErr.Code == "23505":
		return fmt.Errorf("%w: %s", storage.ErrConflict, pgErr.ConstraintName)
	}
	return fmt.Errorf("%w: %w", storage.ErrUnexpected, err)
}

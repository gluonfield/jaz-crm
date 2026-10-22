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

// New opens the store on pool, in schema or, when schema is empty, wherever
// the pool's connections look. It fails while migrations are pending: an
// unmigrated schema would let names resolve to the host's tables in public.
func New(ctx context.Context, pool *pgxpool.Pool, schema string) (*Store, error) {
	provider, err := migrator(pool, schema)
	if err != nil {
		return nil, err
	}
	defer provider.Close()
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("check migrations: %w", err)
	}
	if pending {
		return nil, errors.New("migrations are pending: run Migrate first")
	}
	var db interface {
		beginner
		authdb.DBTX
	} = pool
	if schema != "" {
		db = newScoped(pool, schema)
	}
	return &Store{pool: pool, db: db, auth: authdb.New(db), rec: recdb.New(db), conn: conndb.New(db), in: intdb.New(db), logo: logodb.New(db)}, nil
}

// Migrate brings schema, created if missing, or the pool's own search_path
// when schema is empty, to the latest version. It connects through a pool of
// its own made from pool's configuration and closed on return, so pool's
// connections keep their settings.
func Migrate(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	config := pool.Config()
	if schema != "" {
		if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+pgx.Identifier{schema}.Sanitize()); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
		config.ConnConfig.RuntimeParams["search_path"] = searchPath(schema)
	}
	own, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return err
	}
	defer own.Close()
	// Every process migrates when it starts; a session lock runs one's
	// migrations at a time.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	provider, err := migrator(own, schema, goose.WithSessionLocker(locker))
	if err != nil {
		return err
	}
	defer provider.Close()
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func migrator(pool *pgxpool.Pool, schema string, opts ...goose.ProviderOption) (*goose.Provider, error) {
	dir, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	table := goose.DefaultTablename
	if schema != "" {
		table = schema + "." + table
	}
	opts = append(opts, goose.WithDisableGlobalRegistry(true), goose.WithGoMigrations(datamigrations.All...), goose.WithTableName(table))
	provider, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(pool), dir, opts...)
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return provider, nil
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

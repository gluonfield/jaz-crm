// Package postgrestest gives tests a migrated Postgres database of their own.
package postgrestest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/gluonfield/jaz-crm/internal/storage/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultURL is the docker compose Postgres; override with TEST_DATABASE_URL.
const DefaultURL = "postgres://jazcrm:jazcrm@localhost:55532/jazcrm?sslmode=disable"

// New creates a throwaway database, migrates it and drops it after the test.
func New(t testing.TB) *postgres.Store {
	t.Helper()
	ctx := context.Background()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = DefaultURL
	}
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("postgres unavailable at %s (start it with `docker compose up -d postgres`): %v", base, err)
	}
	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "jazcrm_test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return Open(t, u.String())
}

// Open migrates the database at dsn, as a process does when it starts, and
// returns a store on it whose pool closes after the test.
func Open(t testing.TB, dsn string) *postgres.Store {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	store, err := postgres.New(ctx, pool, "")
	if err != nil {
		t.Fatal(err)
	}
	return store
}

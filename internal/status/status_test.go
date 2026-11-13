package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/internal/status"
	"github.com/gluonfield/jaz-crm/internal/storage"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/internal/workspaces"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStatusIgnoresDeletedConnections(t *testing.T) {
	ctx := t.Context()
	dsn := postgrestest.Database(t)
	store := postgrestest.Open(t, dsn)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	connection, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: owner.WorkspaceID, UserID: owner.ID, Provider: "google", Account: "mail@example.test", RefreshToken: []byte("encrypted")})
	if err != nil {
		t.Fatal(err)
	}
	service := status.NewService(store)
	now := time.Now()
	observation := status.Observation{ObservedAt: now, OccurredAt: &now, Reason: "Sync could not finish."}
	if err := service.Observe(ctx, "GmailDrafts", &connection.ID, observation); err != nil {
		t.Fatal(err)
	}
	if rows, err := service.Errors(ctx, owner.WorkspaceID, owner.ID); err != nil || len(rows) != 1 {
		t.Fatalf("connected account's failure was not recorded: %+v (%v)", rows, err)
	}
	deleting, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer deleting.Rollback(ctx)
	if _, err := deleting.Exec(ctx, "DELETE FROM connections WHERE id = $1", connection.ID); err != nil {
		t.Fatal(err)
	}
	observation.ObservedAt = now.Add(time.Second)
	finished := make(chan error, 1)
	go func() { finished <- service.Observe(ctx, "GmailDrafts", &connection.ID, observation) }()
	waiting, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for {
		var blocked bool
		if err := pool.QueryRow(waiting, "SELECT EXISTS (SELECT FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))", deleting.Conn().PgConn().PID()).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := deleting.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatalf("activity reporting raced with connection deletion: %v", err)
	}
	observation.ObservedAt = now.Add(2 * time.Second)
	if err := service.Observe(ctx, "GmailDrafts", &connection.ID, observation); err != nil {
		t.Fatalf("finished activity could not report its deleted connection: %v", err)
	}
	if rows, err := service.Errors(ctx, owner.WorkspaceID, owner.ID); err != nil || len(rows) != 0 {
		t.Fatalf("deleted connection retained an issue: %+v (%v)", rows, err)
	}
}

func TestStatusNotifiesErrorsAndRecoveryWithoutIdleRefreshes(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	store := postgrestest.New(t)
	people := workspaces.NewService(store, workspaces.Config{})
	first, err := people.Provision(ctx, "first@example.test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := people.Provision(ctx, "second@example.test")
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	events := make(chan string, 8)
	done := make(chan error, 1)
	go func() {
		done <- store.Listen(ctx, func() { close(ready) }, func(workspace string) { events <- workspace })
	}()
	select {
	case <-ready:
	case err := <-done:
		cancel()
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-done
	}()
	quiet := func() {
		t.Helper()
		select {
		case workspace := <-events:
			t.Fatalf("idle/stale status refreshed a workspace: %q", workspace)
		case <-time.After(150 * time.Millisecond):
		}
	}
	changed := func() {
		t.Helper()
		seen := map[string]bool{}
		for range 2 {
			select {
			case workspace := <-events:
				if workspace != first.WorkspaceID && workspace != second.WorkspaceID {
					t.Fatalf("notification payload was not a workspace ID: %q", workspace)
				}
				seen[workspace] = true
			case <-time.After(time.Second):
				t.Fatal("reported issue/recovery did not notify both workspaces")
			}
		}
		if len(seen) != 2 {
			t.Fatalf("global status did not reach both workspaces: %v", seen)
		}
	}
	service := status.NewService(store)
	now := time.Now().Truncate(time.Second)
	observe := func(observation status.Observation) {
		t.Helper()
		if err := service.Observe(ctx, status.AI, nil, observation); err != nil {
			t.Fatal(err)
		}
	}
	observe(status.Observation{})
	observe(status.Observation{ObservedAt: now})
	quiet()
	failed := status.Observation{ObservedAt: now.Add(time.Second), OccurredAt: new(now.Add(time.Second)), Reason: "Check the model provider's credit or usage limit."}
	observe(failed)
	changed()
	observe(status.Observation{ObservedAt: now.Add(2 * time.Second)})
	changed()
	observe(status.Observation{ObservedAt: now.Add(3 * time.Second)})
	observe(failed)
	quiet()
	if rows, err := service.Errors(ctx, first.WorkspaceID, first.ID); err != nil || len(rows) != 0 {
		t.Fatalf("old failure reappeared after newer recovery: %+v (%v)", rows, err)
	}
}

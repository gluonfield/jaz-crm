package status_test

import (
	"context"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/internal/status"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/internal/workspaces"
)

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

package worker

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/internal/status"
	"github.com/gluonfield/jaz-crm/internal/storage"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/internal/workspaces"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	tworker "go.temporal.io/sdk/worker"
)

type availabilitySample struct{ state status.Observation }

func (s *availabilitySample) Availability() status.Observation { return s.state }

type unavailableStatusStore struct{ storage.OperationStore }

func (unavailableStatusStore) ObserveOperation(context.Context, storage.OperationStatus) error {
	return errors.New("status database unavailable")
}

func TestActivityStatusPreservesResultsAndObservedAIState(t *testing.T) {
	ctx := t.Context()
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	connection, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: owner.WorkspaceID, UserID: owner.ID, Provider: "google", Account: "private-account@example.test", RefreshToken: []byte("encrypted")})
	if err != nil {
		t.Fatal(err)
	}
	service := status.NewService(store)
	now := time.Now()
	paused := status.Observation{ObservedAt: now, OccurredAt: &now, Reason: "AI work is paused. Check the model provider's credit or usage limit.", RetryAt: new(now.Add(time.Hour))}
	if err := service.Observe(ctx, status.AI, nil, paused); err != nil {
		t.Fatal(err)
	}
	ai := &availabilitySample{}
	var logs bytes.Buffer
	observer := NewStatusObserver(StatusParams{Status: service, Connections: store, Logger: log.New(&logs), AI: ai})
	failed := false
	cancelled := false
	run := func() error {
		t.Helper()
		var suite testsuite.WorkflowTestSuite
		env := suite.NewTestActivityEnvironment()
		env.SetWorkerOptions(tworker.Options{Interceptors: []interceptor.WorkerInterceptor{observer}})
		env.RegisterActivityWithOptions(func(context.Context, string) (string, error) {
			if cancelled {
				return "", context.Canceled
			}
			if failed {
				return "", errors.New("private provider payload")
			}
			return "original result", nil
		}, activity.RegisterOptions{Name: "GmailDrafts"})
		result, err := env.ExecuteActivity("GmailDrafts", connection.ID)
		if err == nil {
			var value string
			if err := result.Get(&value); err != nil || value != "original result" {
				t.Fatalf("observer changed the activity result: %q (%v)", value, err)
			}
		}
		return err
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if rows, err := service.Errors(ctx, owner.WorkspaceID, owner.ID); err != nil || len(rows) != 1 {
		t.Fatalf("unknown new worker cleared AI pause: %+v (%v)", rows, err)
	}
	ai.state = paused
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if rows, err := service.Errors(ctx, owner.WorkspaceID, owner.ID); err != nil || len(rows) != 1 || !rows[0].OccurredAt.Equal(now.Truncate(time.Microsecond)) {
		t.Fatalf("nil activity result cleared a deferred AI issue or invented a new occurrence: %+v (%v)", rows, err)
	}
	failed = true
	if err := run(); err == nil || !strings.Contains(err.Error(), "private provider payload") {
		t.Fatalf("observer masked the original error: %v", err)
	}
	rows, err := service.Errors(ctx, owner.WorkspaceID, owner.ID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("sync failure not recorded: %+v (%v)", rows, err)
	}
	for _, row := range rows {
		if strings.Contains(row.Reason, "private") || strings.Contains(row.Reason, connection.Account) {
			t.Fatalf("raw activity details reached public status: %+v", row)
		}
	}
	failed = false
	ai.state = status.Observation{ObservedAt: now.Add(time.Minute)}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if rows, err := service.Errors(ctx, owner.WorkspaceID, owner.ID); err != nil || len(rows) != 0 {
		t.Fatalf("actual recoveries left issues active: %+v (%v)", rows, err)
	}
	seen := time.Now()
	if err := service.Observe(ctx, "GmailDrafts", &connection.ID, status.Observation{ObservedAt: seen, OccurredAt: &seen, Reason: "Earlier sync issue."}); err != nil {
		t.Fatal(err)
	}
	cancelled = true
	if err := run(); err == nil {
		t.Fatal("observer swallowed cancellation")
	}
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	observer.record(cancelledCtx, "GmailDrafts", []any{connection.ID}, nil)
	if rows, err := service.Errors(ctx, owner.WorkspaceID, owner.ID); err != nil || len(rows) != 1 || rows[0].Reason != "Earlier sync issue." {
		t.Fatalf("cancellation created or cleared an issue: %+v (%v)", rows, err)
	}
	cancelled = false
	observer.Status = status.NewService(unavailableStatusStore{})
	if err := run(); err != nil {
		t.Fatalf("status persistence failure caused sync retry: %v", err)
	}
	failed = true
	if err := run(); err == nil || !strings.Contains(err.Error(), "private provider payload") || strings.Contains(err.Error(), "status database unavailable") {
		t.Fatalf("status persistence failure replaced the activity error: %v", err)
	}
}

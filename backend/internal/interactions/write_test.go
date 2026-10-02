package interactions_test

import (
	"context"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

type rejectedVerdicts struct {
	storage.InteractionStore
}

func (s rejectedVerdicts) Atomically(ctx context.Context, fn func(storage.InteractionStore) error) error {
	return s.InteractionStore.Atomically(ctx, func(tx storage.InteractionStore) error {
		return fn(rejectedVerdicts{tx})
	})
}

func (s rejectedVerdicts) SetTriage(ctx context.Context, v storage.Verdict) error {
	v.Triage = "invalid"
	return s.InteractionStore.SetTriage(ctx, v)
}

func TestFailedApprovalRollsBackRecordsAndDecision(t *testing.T) {
	e := setup(t, nil)
	svc := interactions.NewService(interactions.Params{Store: rejectedVerdicts{e.store}, Connections: e.store, Workspaces: e.store, Records: e.crm})
	if _, err := svc.Decide(ctx, e.a, interactions.Decision{Addresses: []string{"ada@customer.io"}, Keep: true}); err == nil {
		t.Fatal("Postgres accepted the invalid verdict")
	}
	for _, object := range []string{"people", "companies"} {
		got, _, err := e.crm.Search(ctx, e.a, records.Search{Object: object})
		if err != nil || len(got) != 0 {
			t.Fatalf("failed approval left %s: %+v %v", object, got, err)
		}
	}
	if got := e.contacts(t, interactions.Pending); len(got) != 0 {
		t.Fatalf("failed approval left handles: %+v", got)
	}
}

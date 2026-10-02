package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

func TestInterruptedDraftRetriesWithoutStaleCompletion(t *testing.T) {
	ctx := context.Background()
	db, url := legacy(t, 24)
	store, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "drafting@cas.dev")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	crm := records.NewService(store)
	person, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Jane"}}})
	if err != nil {
		t.Fatal(err)
	}
	service := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	latest := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	conversation, err := service.Log(ctx, actor, "manual", interactions.Entry{Kind: "message", Channel: "linkedin", At: latest.Format(time.RFC3339Nano), Sender: "Jane", Recipients: []string{"August"}, Text: "Does this work with CNC?", Records: []string{person.ID}})
	if err != nil {
		t.Fatal(err)
	}
	started, err := store.ClaimFollowUp(ctx, conversation.ID, nil, &latest)
	if err != nil || started == nil {
		t.Fatalf("claim: %v %v", started, err)
	}
	if other, err := store.ClaimFollowUp(ctx, conversation.ID, &latest, &latest); err != nil || other != nil {
		t.Fatalf("a second worker claimed a live attempt: %v %v", other, err)
	}
	expired := time.Now().Add(-16 * time.Minute).UTC().Truncate(time.Microsecond)
	if _, err := db.ExecContext(ctx, "UPDATE interactions SET drafting_started_at = $1 WHERE id = $2", expired, conversation.ID); err != nil {
		t.Fatal(err)
	}
	view, err := service.Get(ctx, actor, conversation.ID)
	if err != nil || view.Drafting == nil || view.Drafting.State != "failed" || !strings.Contains(view.Drafting.Reason, "interrupted") {
		t.Fatalf("interrupted attempt stays drafting: %+v %v", view.Drafting, err)
	}
	candidates, err := store.FollowUpCandidates(ctx, owner.WorkspaceID, latest.Add(-time.Minute), 10)
	if err != nil || len(candidates) != 1 || candidates[0].ID != conversation.ID {
		t.Fatalf("interrupted work was not retried: %+v %v", candidates, err)
	}
	restarted, err := store.ClaimFollowUp(ctx, conversation.ID, &latest, &latest)
	if err != nil || restarted == nil {
		t.Fatalf("retry claim: %v %v", restarted, err)
	}
	if finished, err := store.FinishFollowUp(ctx, conversation.ID, expired, &latest, "completed", ""); err != nil || finished {
		t.Fatalf("stale completion overwrote the retry: %v %v", finished, err)
	}
	if finished, err := store.FinishFollowUp(ctx, conversation.ID, *restarted, &latest, "failed", "The model service could not connect."); err != nil || !finished {
		t.Fatalf("retry could not report failure: %v %v", finished, err)
	}
	if candidates, err := store.FollowUpCandidates(ctx, owner.WorkspaceID, latest.Add(-time.Minute), 10); err != nil || len(candidates) != 1 {
		t.Fatalf("a failed retry was abandoned: %+v %v", candidates, err)
	}
	restarted, err = store.ClaimFollowUp(ctx, conversation.ID, &latest, &latest)
	if err != nil || restarted == nil {
		t.Fatalf("failed retry claim: %v %v", restarted, err)
	}
	if finished, err := store.FinishFollowUp(ctx, conversation.ID, *restarted, &latest, "skipped", "CNC compatibility has not been confirmed."); err != nil || !finished {
		t.Fatalf("retry could not finish: %v %v", finished, err)
	}
	if candidates, err := store.FollowUpCandidates(ctx, owner.WorkspaceID, latest.Add(-time.Minute), 10); err != nil || len(candidates) != 0 {
		t.Fatalf("completed review was queued again: %+v %v", candidates, err)
	}
}

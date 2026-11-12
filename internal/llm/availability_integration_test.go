package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/interactions"
	"github.com/gluonfield/jaz-crm/internal/followups"
	"github.com/gluonfield/jaz-crm/internal/storage"
	"github.com/gluonfield/jaz-crm/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/internal/workspaces"
	"github.com/gluonfield/jaz-crm/records"
)

func TestPausedAIKeepsWorkPendingAndResumes(t *testing.T) {
	ctx := t.Context()
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@example.test")
	if err != nil {
		t.Fatal(err)
	}
	actor := records.Actor{MemberID: owner.ID, WorkspaceID: owner.WorkspaceID}
	crm := records.NewService(store)
	person, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Customer"}}})
	if err != nil {
		t.Fatal(err)
	}
	var threads []string
	for i := range 2 {
		at := time.Now().Add(-8 * 24 * time.Hour)
		thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: fmt.Sprint(i), UserID: &owner.ID, Title: "Question", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: fmt.Sprint(i), At: &at, Content: new("Please send the price.")}); err != nil {
			t.Fatal(err)
		}
		if err := store.AddLink(ctx, thread, person.ID, "user"); err != nil {
			t.Fatal(err)
		}
		threads = append(threads, thread)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: map[string][]string{"name": {"Reply to customer"}, "person": {person.ID}, "status": {"Open"}}}); err != nil {
		t.Fatal(err)
	}
	handle, err := store.UpsertHandle(ctx, storage.NewHandle{WorkspaceID: actor.WorkspaceID, Kind: "email", Value: "candidate@buyer.test", Triage: interactions.Pending})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateWorkspace(ctx, actor.WorkspaceID, storage.WorkspaceUpdate{Description: new("business relationships")}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateTriageSettings(ctx, actor.WorkspaceID, storage.TriageSettings{AutoKeepAi: true}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var available atomic.Bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if !available.Load() {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"No credits","type":"insufficient_quota","code":"credit_balance_exhausted"}}`)
			return
		}
		var request struct {
			Text struct{ Format struct{ Name string } }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		answer := `{"follow_ups":[],"contexts":[],"skip_reason":"No reply required."}`
		if request.Text.Format.Name == "triage" {
			answer = `{"judgements":[{"address":"candidate@buyer.test","verdict":"skip","reason":"automated"}]}`
		}
		if request.Text.Format.Name == "context" {
			answer = `{"context":"- Customer requested pricing."}`
		}
		fmt.Fprintf(w, `{"id":"resp_ok","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":%q}]}]}`, answer)
	}))
	t.Cleanup(api.Close)
	var logs bytes.Buffer
	logger := log.New(&logs)
	client := New(Config{BaseURL: api.URL, APIKey: "key", Model: "gpt-6-luna", Effort: "medium"}, logger)
	now := time.Now()
	client.now = func() time.Time { return now }
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm, Classifier: client})
	agent := followups.NewAgent(followups.AgentParams{Service: followups.NewService(crm, store, nil, store), Workspaces: store, Interactions: convs, Logger: logger, Planner: client, Summarizer: client})
	if _, err := agent.Run(ctx, actor.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	for i, thread := range threads {
		at := time.Now().Add(-time.Minute)
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: fmt.Sprint("new", i), At: &at, Content: new("What is the new price?")}); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		if err := convs.Triage(ctx, actor.WorkspaceID); err != nil {
			t.Fatal(err)
		}
		if count, err := agent.Run(ctx, actor.WorkspaceID); err != nil || count != 0 {
			t.Fatalf("paused follow-ups: count=%d err=%v", count, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("paused jobs uploaded %d requests", calls.Load())
	}
	before, err := crm.Get(ctx, actor, person.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range before.Fields {
		if field.Attribute == records.ContextAttribute && len(field.Values) != 0 {
			t.Fatalf("paused context backfill consumed work: %+v", field)
		}
	}
	pending, err := store.UnassessedHandles(ctx, actor.WorkspaceID, 25)
	if err != nil || len(pending) != 1 || pending[0].ID != handle.ID {
		t.Fatalf("paused triage stamped work as assessed: %+v (%v)", pending, err)
	}
	rows, err := store.Interactions(ctx, actor.WorkspaceID, threads)
	if err != nil || len(rows) != 2 {
		t.Fatalf("pending threads: %+v (%v)", rows, err)
	}
	for _, row := range rows {
		if row.FollowedUpAt != nil || row.DraftingState == "drafting" {
			t.Fatalf("paused follow-up consumed work or retained claim: %+v", row)
		}
	}
	if bytes.Contains(logs.Bytes(), []byte("conversation not followed up")) || bytes.Contains(logs.Bytes(), []byte("contexts not written")) {
		t.Fatalf("paused jobs repeated warnings: %s", logs.String())
	}
	available.Store(true)
	now = now.Add(time.Hour)
	if err := convs.Triage(ctx, actor.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	if count, err := agent.Run(ctx, actor.WorkspaceID); err != nil || count != 2 {
		t.Fatalf("pending follow-ups did not resume: count=%d err=%v", count, err)
	}
	handles, err := store.Handles(ctx, actor.WorkspaceID, []string{handle.ID})
	if err != nil || len(handles) != 1 || handles[0].Triage != interactions.Skipped {
		t.Fatalf("pending triage did not resume: %+v (%v)", handles, err)
	}
	rows, err = store.Interactions(ctx, actor.WorkspaceID, threads)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.FollowedUpAt == nil || row.DraftingState != "skipped" {
			t.Fatalf("recovered follow-up was not processed: %+v", row)
		}
	}
	after, err := crm.Get(ctx, actor, person.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range after.Fields {
		if field.Attribute == records.ContextAttribute && len(field.Values) == 1 && field.Values[0].Text == "- Customer requested pricing." {
			return
		}
	}
	t.Fatalf("pending context backfill did not resume: %+v", after)
}

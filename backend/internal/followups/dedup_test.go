package followups_test

import (
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

func TestAgentReusesConversationReplyAndRollsBackFailedPlan(t *testing.T) {
	ctx := t.Context()
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "writer@company.test")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	crm := records.NewService(store)
	person, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Jane"}}})
	if err != nil {
		t.Fatal(err)
	}
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	brain := &planner{}
	agent := followups.NewAgent(followups.AgentParams{Service: followups.NewService(crm, store, nil, store), Workspaces: store, Interactions: convs, Logger: log.New(io.Discard), Planner: brain})
	at := time.Now().Add(-time.Minute)
	thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "quote", UserID: &owner.ID, Title: "Quote", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddLink(ctx, thread, person.ID, "user"); err != nil {
		t.Fatal(err)
	}
	firstThread := thread
	message := func(id string) {
		t.Helper()
		at = at.Add(time.Second)
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: id, At: &at, Content: new("Could you confirm the quote?")}); err != nil {
			t.Fatal(err)
		}
	}
	run := func(plan followups.Plan, want int) {
		t.Helper()
		brain.plans = []followups.Plan{plan}
		if count, err := agent.Run(ctx, actor.WorkspaceID); err != nil || count != want {
			t.Fatalf("run: count=%d err=%v, want %d", count, err, want)
		}
	}
	all := func(want int) []records.Record {
		t.Helper()
		rows, _, err := crm.Search(ctx, actor, records.Search{Object: records.FollowUps})
		if err != nil || len(rows) != want {
			t.Fatalf("follow-ups: got %d, want %d (%v)", len(rows), want, err)
		}
		return rows
	}
	field := func(record records.Record, attribute string) string {
		for _, f := range record.Fields {
			if f.Attribute == attribute && len(f.Values) > 0 {
				return f.Values[0].Text
			}
		}
		return ""
	}
	message("first")
	run(followups.Plan{FollowUps: []followups.Change{
		{Action: "Reply with the quote", WaitingOn: "Us", Status: "Open", Reply: "We are preparing your quote."},
		{Action: "Prepare technical drawings by Friday", WaitingOn: "Us", Status: "Open", Person: person.ID},
	}}, 1)
	var replyID, taskID string
	for _, row := range all(2) {
		if field(row, "draft") != "" {
			replyID = row.ID
		} else {
			taskID = row.ID
		}
	}
	if replyID == "" || taskID == "" {
		t.Fatal("the reply and separate deliverable were not created")
	}
	message("second")
	run(followups.Plan{FollowUps: []followups.Change{{Action: "Confirm pricing and answer Jane", WaitingOn: "Us", Status: "Open", Reply: "Your quote is ready."}}}, 1)
	for _, row := range all(2) {
		if row.ID == replyID && field(row, "draft") != "Your quote is ready." {
			t.Fatalf("the existing reply was not updated: %+v", row)
		}
		if row.ID == taskID && field(row, "name") != "Prepare technical drawings by Friday" {
			t.Fatalf("the separate deliverable was overwritten: %+v", row)
		}
	}
	found := false
	for _, open := range brain.read[len(brain.read)-1].FollowUps {
		found = found || open.ID == replyID && open.Conversation && open.Channel == "Email"
	}
	if !found {
		t.Fatal("a directly linked reply without a person reference was missing from open_follow_ups")
	}
	message("third")
	run(followups.Plan{SkipReason: "Delivery needs a separate action.", FollowUps: []followups.Change{
		{Action: "Arrange delivery", WaitingOn: "Us", Status: "Open", Person: person.ID},
		{Action: "Invalid change", WaitingOn: "Us", Status: "Invalid"},
	}}, 0)
	all(2)
	run(followups.Plan{SkipReason: "Delivery needs a separate action.", FollowUps: []followups.Change{{Action: "Arrange delivery", WaitingOn: "Us", Status: "Open", Person: person.ID}}}, 1)
	all(3)
	other, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "other", UserID: &owner.ID, Title: "Another project", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddLink(ctx, other, person.ID, "user"); err != nil {
		t.Fatal(err)
	}
	thread = other
	message("other-message")
	run(followups.Plan{FollowUps: []followups.Change{{Action: "Reply about the other project", WaitingOn: "Us", Status: "Open", Person: person.ID, Reply: "Let's discuss the other project."}}}, 1)
	all(4)
	connection, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: owner.WorkspaceID, UserID: owner.ID, Provider: "google", Account: owner.Email, RefreshToken: []byte("encrypted")})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertGmailDraft(ctx, storage.GmailDraft{ConnectionID: connection.ID, DraftID: "gmail-draft", MessageID: "gmail-message", Body: "Your quote is ready.", State: "draft"}); err != nil {
		t.Fatal(err)
	}
	if err := store.BindGmailDraft(ctx, connection.ID, "gmail-draft", replyID); err != nil {
		t.Fatal(err)
	}
	thread = firstThread
	message("fourth")
	run(followups.Plan{FollowUps: []followups.Change{
		{ID: replyID, Status: "Done"},
		{Action: "Draft another reply", WaitingOn: "Us", Status: "Open", Reply: "An automatically replaced reply."},
	}}, 1)
	for _, row := range all(4) {
		if row.ID == replyID && (field(row, "status") != "Open" || field(row, "draft") != "Your quote is ready.") {
			t.Fatalf("the agent overwrote or closed a Gmail draft: %+v", row)
		}
	}
	var otherReply string
	for _, row := range all(4) {
		if field(row, "draft") == "Let's discuss the other project." {
			otherReply = row.ID
		}
	}
	thread = other
	message("another-message")
	brain.onPlan = func() {
		if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: otherReply, Set: map[string][]string{"status": {"Done"}}}); err != nil {
			t.Fatal(err)
		}
	}
	run(followups.Plan{FollowUps: []followups.Change{{Action: "Send stale reply", WaitingOn: "Us", Status: "Open", Person: person.ID, Reply: "This action was completed while I was thinking."}}}, 1)
	all(4)
}

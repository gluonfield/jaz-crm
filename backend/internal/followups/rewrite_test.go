package followups_test

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
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
	signin "github.com/gluonfield/jaz-tasks/auth"
)

type rewriter struct {
	inputs []followups.RewriteInput
}

func (r *rewriter) Rewrite(_ context.Context, input followups.RewriteInput) (followups.RewriteResult, error) {
	r.inputs = append(r.inputs, input)
	return followups.RewriteResult{Draft: "A proposed revision", Subject: input.Subject}, nil
}

func TestRewriteUsesStoredContextWithoutSaving(t *testing.T) {
	ctx := t.Context()
	store := postgrestest.New(t)
	workspaces := workspaces.NewService(store, workspaces.Config{})
	owner, err := workspaces.SignIn(ctx, signin.Identity{Issuer: "test", Subject: "owner", Email: "owner@rewrite.test", EmailVerified: true, Name: "Alex Writer"})
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	brain := &rewriter{}
	agent := followups.NewAgent(followups.AgentParams{Service: followups.NewService(crm, store, nil, store), Workspaces: store, Interactions: convs, Logger: log.New(io.Discard), Rewriter: brain})
	write := func(object string, fields map[string][]string) records.Record {
		t.Helper()
		record, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: object, Set: fields})
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	page := write(records.Pages, map[string][]string{"name": {"Capabilities"}, "content": {"We can quote aluminium brackets."}})
	child := write(records.Pages, map[string][]string{"name": {"Booking"}, "parent": {page.ID}, "content": {"Use https://example.test/book for a call."}})
	if _, err := workspaces.Update(ctx, actor, storage.WorkspaceUpdate{CompanyPageIDs: []string{page.ID}, DraftingWebAccess: new(true)}); err != nil {
		t.Fatal(err)
	}
	company := write("companies", map[string][]string{"name": {"Customer"}})
	person := write("people", map[string][]string{"name": {"Jane"}, "company": {company.ID}, "context": {"Met at a trade show."}})
	recipient := write("people", map[string][]string{"name": {"Sam"}, "email_addresses": {"sam@customer.test"}, "context": {"Existing contact entered in the composer."}})
	f := write(records.FollowUps, map[string][]string{"name": {"Reply to Jane"}, "person": {person.ID}, "channel": {"LinkedIn"}, "draft": {"The saved draft"}})
	input := followups.RewriteInput{Draft: "My unsaved edit", Action: "shorten", To: []string{"Sam <SAM@customer.test>"}}
	if _, err := agent.Rewrite(ctx, actor, f.ID, input); err != nil {
		t.Fatal(err)
	}
	initial := brain.inputs[0]
	if initial.Draft != input.Draft || initial.Context.Sender == nil || initial.Context.Sender.Address != owner.Email || initial.Context.WebAccess {
		t.Fatalf("new outreach must carry the unsaved draft and sender without enabling web: %+v", initial)
	}
	if len(initial.Context.Company) != 2 || !slices.ContainsFunc(initial.Context.Company, func(r followups.Record) bool { return r.ID == child.ID && r.Path == "Capabilities / Booking" }) {
		t.Fatal("selected company knowledge and descendants were not included")
	}
	for _, id := range []string{person.ID, recipient.ID, company.ID, f.ID} {
		if !slices.ContainsFunc(initial.Context.Records, func(r followups.Record) bool { return r.ID == id }) {
			t.Fatalf("missing related record %s", id)
		}
	}
	long := strings.Repeat("Original requirements, including quoted history. λ\n", 1000)
	for i := range 103 {
		at := time.Now().Add(-time.Duration(i+1) * time.Hour)
		thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: fmt.Sprintf("rewrite-%d", i), UserID: &owner.ID, Title: "Quote", At: at})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: "body", At: &at, Content: &long}); err != nil {
			t.Fatal(err)
		}
		linked := person.ID
		if i == 0 || i == 101 {
			linked = f.ID
		} else if i == 102 {
			linked = recipient.ID
		}
		if err := store.AddLink(ctx, thread, linked, "user"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := agent.Rewrite(ctx, actor, f.ID, input); err != nil {
		t.Fatal(err)
	}
	full := brain.inputs[1].Context
	if len(full.Messages) != 1 || full.Messages[0].Text != long || len(full.History) != 102 {
		t.Fatalf("current thread and every contact/follow-up history page must survive: messages=%d history=%d", len(full.Messages), len(full.History))
	}
	for _, previous := range full.History {
		if len(previous.Messages) != 1 || previous.Messages[0].Text != long {
			t.Fatal("historical content was cut")
		}
	}
	stored, err := crm.Get(ctx, actor, f.ID)
	if err != nil || !reflect.DeepEqual(stored, f) {
		t.Fatalf("a rewrite proposal changed the saved draft: %+v %v", stored, err)
	}
	connection, err := store.SaveConnection(ctx, storage.NewConnection{WorkspaceID: actor.WorkspaceID, UserID: actor.UserID, Provider: "google", Account: "alex@alternate.test", RefreshToken: []byte("unused")})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddAliases(ctx, connection.ID, []string{"sales@alternate.test"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetConnectionStatus(ctx, connection.ID, "revoked"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, RecordID: f.ID, Set: map[string][]string{"channel": {"Email"}}}); err != nil {
		t.Fatal(err)
	}
	input.From = "sales@alternate.test"
	if _, err := agent.Rewrite(ctx, actor, f.ID, input); err != nil {
		t.Fatal(err)
	}
	if sender := brain.inputs[2].Context.Sender; sender == nil || sender.Address != input.From || sender.Name != owner.Name {
		t.Fatalf("an alternate sender must keep their known name, even without active send access: %+v", sender)
	}
	other, err := workspaces.Provision(ctx, "other@isolated.test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Rewrite(ctx, auth.Actor{UserID: other.ID, WorkspaceID: other.WorkspaceID}, f.ID, input); err == nil || len(brain.inputs) != 3 {
		t.Fatal("another workspace read the draft or called the model")
	}
	if _, err := agent.Rewrite(ctx, actor, person.ID, input); err == nil {
		t.Fatal("rewrote a non-follow-up")
	}
	for _, invalid := range []followups.RewriteInput{{Draft: "text", Action: "unknown"}, {Action: "shorten"}, {Draft: "text", Action: "custom"}} {
		if _, err := agent.Rewrite(ctx, actor, f.ID, invalid); err == nil || len(brain.inputs) != 3 {
			t.Fatal("invalid rewrite reached the model")
		}
	}
	if _, err := agent.Rewrite(ctx, actor, f.ID, followups.RewriteInput{Action: "write"}); err != nil || len(brain.inputs) != 4 || !slices.ContainsFunc(brain.inputs[3].Context.Records, func(r followups.Record) bool { return r.ID == f.ID }) {
		t.Fatalf("a first draft must reach the model with its follow-up: %v", err)
	}
}

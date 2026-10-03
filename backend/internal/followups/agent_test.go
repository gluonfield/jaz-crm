package followups_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/google"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	signin "github.com/gluonfield/jaz-tasks/auth"
)

// planner stands in for the model: it records what it read and answers with
// the next plan, or fails.
type planner struct {
	read      []followups.Conversation
	plans     []followups.Plan
	fail      bool
	onPlan    func()
	histories []followups.History
	summary   string
}

func (p *planner) Summarize(_ context.Context, h followups.History) (string, error) {
	p.histories = append(p.histories, h)
	return p.summary, nil
}

func (p *planner) Plan(_ context.Context, c followups.Conversation, _ followups.ReadTools) (followups.Plan, error) {
	p.read = append(p.read, c)
	if p.onPlan != nil {
		p.onPlan()
	}
	if p.fail {
		return followups.Plan{}, errors.New("model unavailable")
	}
	plan := p.plans[0]
	p.plans = p.plans[1:]
	return plan, nil
}

func TestCancelledDraftReleasesItsClaim(t *testing.T) {
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
	at := time.Now().Add(-time.Minute)
	thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "cancelled", UserID: &owner.ID, Title: "Quote", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: "latest", At: &at, Content: new("Please send the price.")}); err != nil {
		t.Fatal(err)
	}
	if err := store.AddLink(ctx, thread, person.ID, "user"); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	brain := &planner{fail: true, onPlan: cancel}
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	agent := followups.NewAgent(followups.AgentParams{Service: followups.NewService(crm, store, nil, store), Workspaces: store, Interactions: convs, Logger: log.New(io.Discard), Planner: brain})
	if count, err := agent.Run(runCtx, actor.WorkspaceID); count != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled run: count=%d err=%v", count, err)
	}
	rows, err := store.Interactions(ctx, actor.WorkspaceID, []string{thread})
	if err != nil || len(rows) != 1 || rows[0].DraftingState != "failed" || rows[0].FollowedUpAt != nil {
		t.Fatalf("cancelled draft must release its claim and remain unread: %+v (%v)", rows, err)
	}
	brain.fail, brain.onPlan = false, nil
	brain.plans = []followups.Plan{{SkipReason: "The price needs confirmation."}}
	if count, err := agent.Run(ctx, actor.WorkspaceID); err != nil || count != 1 {
		t.Fatalf("cancelled draft must be immediately retryable: count=%d err=%v", count, err)
	}
}

func TestAgentKeepsFollowUpsCurrent(t *testing.T) {
	store := postgrestest.New(t)
	owner, err := workspaces.NewService(store, workspaces.Config{}).Provision(ctx, "owner@cas.dev")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	workspaceService := workspaces.NewService(store, workspaces.Config{})
	zone := "Europe/London"
	if _, err := workspaceService.Update(ctx, actor, storage.WorkspaceUpdate{Timezone: &zone}); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceService.Invite(ctx, actor, "bob@acme.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceService.SignIn(ctx, signin.Identity{Issuer: "test", Subject: "bob", Email: "bob@acme.com", EmailVerified: true}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			fmt.Fprint(w, `{"access_token":"at","token_type":"Bearer","expires_in":3600,"refresh_token":"rt"}`)
		case "/gmail/v1/users/me/profile":
			fmt.Fprint(w, `{"emailAddress":"owner@cas.dev","historyId":"1"}`)
		case "/gmail/v1/users/me/messages/gm2":
			fmt.Fprint(w, `{"id":"gm2","threadId":"t9","internalDate":"0","payload":{"headers":[
				{"name":"From","value":"Jane <jane@acme.com>"},{"name":"To","value":"owner@cas.dev"},
				{"name":"Cc","value":"bob@acme.com"},{"name":"Message-ID","value":"<m2@acme.com>"}]}}`)
		default:
			fmt.Fprint(w, `{"id":"g","threadId":"t9","internalDate":"0","payload":{"headers":[
				{"name":"From","value":"Jane <jane@acme.com>"},{"name":"To","value":"owner@cas.dev, Sam <sam@acme.com>"},
				{"name":"Cc","value":"bob@acme.com, owner@cas.dev, sales@cas.dev"},{"name":"Message-ID","value":"<m@acme.com>"}]}}`)
		}
	}))
	t.Cleanup(srv.Close)
	conns, err := connections.NewService(store, connections.Config{
		Google:    google.OAuthConfig{ClientID: "client", ClientSecret: "secret", TokenURL: srv.URL + "/token"},
		Key:       []byte("0123456789abcdef0123456789abcdef"),
		Endpoints: google.Endpoints{Gmail: srv.URL},
	}, syncer{})
	if err != nil {
		t.Fatal(err)
	}
	mailbox, err := conns.Connect(ctx, actor, "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddAliases(ctx, mailbox.ID, []string{"sales@cas.dev"}); err != nil {
		t.Fatal(err)
	}
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	brain := &planner{}
	agent := followups.NewAgent(followups.AgentParams{Service: followups.NewService(crm, store, conns, store), Workspaces: store, Interactions: convs, Logger: log.New(io.Discard), Planner: brain, Summarizer: brain})
	jane, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Jane"}, "context": {"- Head of purchasing at Acme"}}})
	if err != nil {
		t.Fatal(err)
	}
	deal, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "deals", Set: map[string][]string{"name": {"Acme brackets"}, "people": {jane.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: owner.WorkspaceID, ExternalID: "t9", ConnectionID: &mailbox.ID, UserID: &owner.ID, Title: "Quote", At: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddLink(ctx, thread, jane.ID, "user"); err != nil {
		t.Fatal(err)
	}
	message := func(id, text string, at time.Time) {
		provider := "g" + id
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: id, ConnectionID: &mailbox.ID, ProviderID: &provider, At: &at, Content: &text}); err != nil {
			t.Fatal(err)
		}
	}
	state := func(want, reason string) {
		t.Helper()
		conv, err := convs.Get(ctx, actor, thread)
		if err != nil || conv.Drafting == nil || conv.Drafting.State != want || conv.Drafting.StartedAt == nil || !strings.Contains(conv.Drafting.Reason, reason) {
			t.Fatalf("drafting state: %+v, %v; want %s with %q", conv.Drafting, err, want, reason)
		}
	}
	brain.onPlan = func() {
		state("drafting", "")
	}
	run := func() int {
		read, err := agent.Run(ctx, owner.WorkspaceID)
		if err != nil {
			t.Fatal(err)
		}
		return read
	}
	message("old", "Last month's thread.", time.Now().Add(-10*24*time.Hour))
	if run() != 0 || len(brain.read) != 0 {
		t.Fatal("content older than a week is history, not news")
	}

	message("m1", "Could you send a revised quote for 500 brackets by Friday?", time.Now().Add(-time.Minute))
	merged := "- Head of purchasing at Acme\n- 2026-10-01: asked for a revised quote for 500–600 brackets by Friday"
	brain.plans = []followups.Plan{{FollowUps: []followups.Change{
		{Action: "Send revised quote for 500 brackets", WaitingOn: "Us", ActionDate: &followups.ActionDate{Value: "2026-10-25T10:00", Basis: "Stated", Reason: "October 25 at 10 local time"}, Status: "Open", Person: jane.ID, Company: "made-up", Reply: "Hi Jane, here is the revised quote."},
		{ID: "invented", Status: "Done"},
	}, Contexts: []followups.Context{{Person: jane.ID, Context: merged}, {Person: deal.ID, Context: "- not a person in this conversation"}}}}
	if run() != 1 {
		t.Fatal("the new message was not read")
	}
	state("completed", "")
	if !slices.ContainsFunc(brain.read[0].Records, func(r followups.Record) bool {
		return r.ID == jane.ID && r.Object == "people" && slices.Equal(r.Values["context"], []string{"- Head of purchasing at Acme"})
	}) {
		t.Fatalf("the planner must read the current context in each person's record: %+v", brain.read[0].Records)
	}
	if person, err := crm.Get(ctx, actor, jane.ID); err != nil || !slices.ContainsFunc(person.Fields, func(f records.Field) bool {
		return f.Attribute == "context" && f.Values[0].Text == strings.Replace(merged, "500–600", "500-600", 1)
	}) {
		t.Fatalf("the merged context must replace the one a person wrote, keeping its bullets and hyphenating ranges: %+v %v", person.Fields, err)
	}
	if got := brain.read[0]; got.Timezone != "Europe/London" || got.Now == "" || got.StartedAt == "" || got.Channel != "email" || len(got.Messages) != 2 || got.Messages[1].Text != "Could you send a revised quote for 500 brackets by Friday?" {
		t.Fatalf("planner read %+v", got)
	}
	if !slices.ContainsFunc(brain.read[0].Records, func(r followups.Record) bool { return r.ID == deal.ID && r.Object == "deals" }) {
		t.Fatalf("the planner must see the deals of the people in the conversation: %+v", brain.read[0].Records)
	}
	open, _, err := crm.Search(ctx, actor, records.Search{Object: records.FollowUps})
	if err != nil || len(open) != 1 {
		t.Fatalf("follow-ups: %v %v", open, err)
	}
	f := open[0]
	values := map[string][]string{}
	for _, field := range f.Fields {
		for _, v := range field.Values {
			values[field.Attribute] = append(values[field.Attribute], v.Text+v.RecordID)
		}
	}
	want := map[string][]string{
		"name": {"Send revised quote for 500 brackets"}, "status": {"Open"}, "waiting_on": {"Us"}, "action_date": {"2026-10-25T10:00:00Z"},
		"owner": {"owner@cas.dev"}, "person": {"Jane" + jane.ID}, "draft": {"Hi Jane, here is the revised quote."},
		"action_date_basis": {"Stated"}, "action_date_reason": {"October 25 at 10 local time"}, "action_date_source": {thread},
		"channel": {"Email"}, "to": {"jane@acme.com", "sam@acme.com"}, "cc": {"bob@acme.com"}, "draft_status": {records.DraftWritten},
	}
	for attr, w := range want {
		if !slices.Equal(values[attr], w) {
			t.Errorf("%s = %v, want %v", attr, values[attr], w)
		}
	}
	if values["company"] != nil {
		t.Errorf("a record the planner was not shown was attached: %v", values["company"])
	}
	if linked, err := convs.Timeline(ctx, actor, f.ID, nil, "", false, 5); err != nil || len(linked) != 1 || linked[0].ID != thread {
		t.Fatalf("the conversation must be linked to its follow-up: %+v %v", linked, err)
	}
	if run() != 0 || len(brain.read) != 1 {
		t.Fatal("a conversation read once was read again")
	}

	message("m2", "Thanks, got the quote. We will confirm next week.", time.Now())
	brain.fail = true
	if run() != 0 {
		t.Fatal("a conversation the model could not plan counted as read")
	}
	state("failed", "model could not complete")
	brain.fail = false
	brain.plans = []followups.Plan{{FollowUps: []followups.Change{{ID: f.ID, Action: "Confirm the order — next week", Status: "Done", Reply: "Thanks Jane—speak next week. Delivery is October 7–8 -- see you then.\n\n—Augustinas"}}}}
	if run() != 1 {
		t.Fatal("a conversation the model could not plan must be read again")
	}
	state("skipped", "generated reply could not")
	if seen := brain.read[len(brain.read)-1].FollowUps; len(seen) != 1 || seen[0].ID != f.ID {
		t.Fatalf("the planner must see the open follow-up: %+v", seen)
	}
	done, err := crm.Get(ctx, actor, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	after := map[string][]string{}
	for _, field := range done.Fields {
		for _, v := range field.Values {
			after[field.Attribute] = append(after[field.Attribute], v.Text)
		}
	}
	if !slices.Equal(after["status"], []string{"Done"}) || len(after["action_date"]) != 0 || len(after["draft"]) != 0 {
		t.Fatalf("a closed follow-up retains an active date or AI draft: %v", after)
	}
	if !slices.Equal(after["name"], []string{"Confirm the order, next week"}) {
		t.Fatalf("actions must be written without dashes: %q", after["name"])
	}
	message("m3", "Does this work with CNC?", time.Now().Add(time.Millisecond))
	brain.plans = []followups.Plan{{}}
	if run() != 0 {
		t.Fatal("an empty model result must remain eligible for retry")
	}
	state("failed", "no reply or explanation")
	brain.plans = []followups.Plan{{SkipReason: "CNC compatibility has not been confirmed."}}
	if run() != 1 {
		t.Fatal("a reviewed conversation without a reply must finish")
	}
	state("skipped", "CNC compatibility has not been confirmed.")
	if run() != 0 {
		t.Fatal("an intentionally skipped draft must not retry without new input")
	}

	if len(brain.histories) != 0 {
		t.Fatalf("a person with a context was summarised again: %+v", brain.histories)
	}
	month := time.Now().Add(-30 * 24 * time.Hour)
	older, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: owner.WorkspaceID, ExternalID: "t-old", ConnectionID: &mailbox.ID, UserID: &owner.ID, Title: "Brackets order", At: month})
	if err != nil {
		t.Fatal(err)
	}
	order, provider := "We will order 2,000 brackets in Q1.", "gold"
	if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: older, Kind: "message", ExternalID: "old@acme.com", ConnectionID: &mailbox.ID, ProviderID: &provider, At: &month, Content: &order}); err != nil {
		t.Fatal(err)
	}
	var people []string
	for _, name := range []string{"Bob", "Carol"} {
		p, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {name}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AddLink(ctx, older, p.ID, "user"); err != nil {
			t.Fatal(err)
		}
		people = append(people, p.ID)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: map[string][]string{"name": {"Ask Bob about the Q1 order"}, "person": {people[0]}}}); err != nil {
		t.Fatal(err)
	}
	dave, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: "people", Set: map[string][]string{"name": {"Dave"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.FollowUps, Set: map[string][]string{"name": {"Introduce Dave"}, "person": {dave.ID}}}); err != nil {
		t.Fatal(err)
	}
	brain.summary = "- Buys brackets for Acme\n- 2026-09-02: plans to order 2,000 brackets in Q1"
	run()
	run()
	contextOf := func(id string) string {
		p, err := crm.Get(ctx, actor, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range p.Fields {
			if f.Attribute == "context" {
				return f.Values[0].Text
			}
		}
		return ""
	}
	if len(brain.histories) != 1 || brain.histories[0].Name != "Bob" || len(brain.histories[0].Conversations) != 1 || brain.histories[0].Conversations[0].Lines[0].Text != order {
		t.Fatalf("a person with an open follow-up and no context must be summarised once from their conversations, past one with none: %+v", brain.histories)
	}
	if contextOf(people[0]) != brain.summary || contextOf(people[1]) != "" {
		t.Fatalf("only the person with an open follow-up gets a context: %q %q", contextOf(people[0]), contextOf(people[1]))
	}
}

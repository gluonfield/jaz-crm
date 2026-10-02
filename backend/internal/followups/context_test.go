package followups_test

import (
	"encoding/json"
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
	"github.com/gluonfield/jaz-crm/backend/internal/followups"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/llm"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
)

func TestDraftRequestIncludesFullConversationAndCompanyKnowledge(t *testing.T) {
	ctx := t.Context()
	store := postgrestest.New(t)
	ws := workspaces.NewService(store, workspaces.Config{})
	owner, err := ws.Provision(ctx, "writer@our-company.test")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	write := func(as auth.Actor, object string, fields map[string][]string) records.Record {
		t.Helper()
		r, _, err := crm.Upsert(ctx, as, records.SourceUser, records.Write{Object: object, Set: fields})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	companyText := "Our company capabilities:\n" + strings.Repeat("We machine aluminium components. λ\n", 1000) + "END CAPABILITIES"
	company := write(actor, records.Pages, map[string][]string{"name": {"Company"}, "content": {companyText}})
	wantDocuments := map[string]string{company.ID: companyText}
	strategy := write(actor, records.Pages, map[string][]string{"name": {"Strategy"}, "parent": {company.ID}, "content": {"Internal roadmap, not delivered capabilities."}})
	wantDocuments[strategy.ID] = "Internal roadmap, not delivered capabilities."
	for i := range 101 {
		text := fmt.Sprintf("Complete product specification %d.", i)
		page := write(actor, records.Pages, map[string][]string{"name": {fmt.Sprintf("Product %d", i)}, "parent": {strategy.ID}, "content": {text}})
		wantDocuments[page.ID] = text
	}
	write(actor, records.Pages, map[string][]string{"name": {"Unrelated notes"}, "content": {"Unrelated document."}})
	other, err := ws.Provision(ctx, "other@another-workspace.test")
	if err != nil {
		t.Fatal(err)
	}
	write(auth.Actor{UserID: other.ID, WorkspaceID: other.WorkspaceID}, records.Pages, map[string][]string{"name": {"Company"}, "content": {"Another workspace's private knowledge."}})
	customer := write(actor, "companies", map[string][]string{"name": {"Customer Ltd"}, "description": {"Builds scientific instruments."}, "size": {"11-50"}})
	person := write(actor, "people", map[string][]string{"name": {"Jane"}, "job_title": {"Purchasing lead"}, "company": {customer.ID}, "context": {"- Asked us about precision brackets."}})
	deal := write(actor, "deals", map[string][]string{"name": {"Bracket order"}, "company": {customer.ID}, "value": {"15000"}})
	followUp := write(actor, records.FollowUps, map[string][]string{"name": {"Answer Jane's question"}, "person": {person.ID}, "company": {customer.ID}, "deal": {deal.ID}})
	thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "complete-thread", UserID: &owner.ID, Title: "Capabilities", At: time.Now().Add(-30 * 24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddLink(ctx, thread, person.ID, "user"); err != nil {
		t.Fatal(err)
	}
	message := func(id, text string, at time.Time) {
		t.Helper()
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: thread, Kind: "message", ExternalID: id, AuthorName: "Jane", Direction: "received", Recipients: []string{owner.Email}, At: &at, Content: &text}); err != nil {
			t.Fatal(err)
		}
	}
	oldText := "Keep our original requirements.\n\nOn Monday, Jane wrote:\n> Quoted requirements must also survive."
	newText := "BEGIN SPECIFICATION\n" + strings.Repeat("Tolerance ±0.01 mm.\n", 2000) + "END SPECIFICATION"
	message("old", oldText, time.Now().Add(-30*24*time.Hour))
	message("new", newText, time.Now().Add(-time.Minute))
	requests := make(chan followups.Conversation, 3)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input string
			Tools []json.RawMessage
		}
		var input followups.Conversation
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal([]byte(req.Input), &input); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(req.Tools) != 0 {
			t.Error("one-shot drafting must not advertise tools")
		}
		requests <- input
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp_1","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"follow_ups\":[],\"contexts\":[]}"}]}]}`)
	}))
	t.Cleanup(api.Close)
	client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "test", Model: "gpt-6-luna", Effort: "medium"})
	agent := followups.NewAgent(followups.AgentParams{Service: followups.NewService(crm, store, nil, store), Workspaces: store, Interactions: convs, Logger: log.New(io.Discard), Planner: client})
	run := func() followups.Conversation {
		t.Helper()
		if count, err := agent.Run(ctx, actor.WorkspaceID); err != nil || count != 1 || len(requests) != 1 {
			t.Fatalf("one changed conversation must produce one model request: count=%d requests=%d err=%v", count, len(requests), err)
		}
		return <-requests
	}
	in := run()
	if len(in.Messages) != 2 || in.Messages[0].Text != oldText || in.Messages[1].Text != newText || in.Messages[1].Author != "Jane" || in.Messages[1].Direction != "received" || !slices.Equal(in.Messages[1].Recipients, []string{owner.Email}) {
		t.Fatal("the model did not receive every original message, in order and without cutting content or headers")
	}
	if in.Sender == nil || in.Sender.Address != owner.Email {
		t.Fatalf("sender identity missing from the actual request: %+v", in.Sender)
	}
	if len(in.Company) != len(wantDocuments) {
		t.Fatalf("Company descendants must be complete and confined to this workspace: got %d want %d", len(in.Company), len(wantDocuments))
	}
	for _, doc := range in.Company {
		if want, ok := wantDocuments[doc.ID]; !ok || !slices.Equal(doc.Values["content"], []string{want}) {
			t.Fatalf("missing, shortened or unrelated company document: %s", doc.Name)
		}
	}
	for id, fields := range map[string]map[string][]string{
		person.ID:   {"job_title": {"Purchasing lead"}, "company": {"Customer Ltd"}},
		customer.ID: {"description": {"Builds scientific instruments."}, "size": {"11-50"}},
		deal.ID:     {"value": {"15000"}},
	} {
		i := slices.IndexFunc(in.Records, func(r followups.Record) bool { return r.ID == id })
		if i < 0 {
			t.Fatalf("related record missing: %s", id)
		}
		for field, want := range fields {
			if !slices.Equal(in.Records[i].Values[field], want) {
				t.Fatalf("%s context missing %s: %+v", in.Records[i].Name, field, in.Records[i].Values)
			}
		}
	}
	if len(in.FollowUps) != 1 || in.FollowUps[0].ID != followUp.ID || len(in.Contexts) != 1 || in.Contexts[0].Person != person.ID {
		t.Fatalf("follow-ups and person context must survive enrichment without duplicates: %+v %+v", in.FollowUps, in.Contexts)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.Pages, RecordID: company.ID, Set: map[string][]string{"content": {"Updated capabilities."}}}); err != nil {
		t.Fatal(err)
	}
	message("latest", "What can you offer now?", time.Now())
	in = run()
	i := slices.IndexFunc(in.Company, func(r followups.Record) bool { return r.ID == company.ID })
	if i < 0 || !slices.Equal(in.Company[i].Values["content"], []string{"Updated capabilities."}) || len(in.Messages) != 3 {
		t.Fatal("a later draft must read current company knowledge and the complete updated thread")
	}
	display, err := convs.Get(ctx, actor, thread)
	if err != nil || display.Messages[0].Text != "Keep our original requirements." {
		t.Fatalf("display cleanup must remain separate from original model input: %v", err)
	}
}

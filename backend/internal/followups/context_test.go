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
	if _, err := ws.Update(ctx, actor, nil, nil, &company.ID); err != nil {
		t.Fatal(err)
	}
	wantDocuments := map[string]string{company.ID: companyText}
	wantPaths := map[string]string{company.ID: "Company"}
	strategy := write(actor, records.Pages, map[string][]string{"name": {"Strategy"}, "parent": {company.ID}, "content": {"Internal roadmap, not delivered capabilities."}})
	wantDocuments[strategy.ID] = "Internal roadmap, not delivered capabilities."
	wantPaths[strategy.ID] = "Company / Strategy"
	for i := range 101 {
		text := fmt.Sprintf("Complete product specification %d.", i)
		page := write(actor, records.Pages, map[string][]string{"name": {fmt.Sprintf("Product %d", i)}, "parent": {strategy.ID}, "content": {text}})
		wantDocuments[page.ID] = text
		wantPaths[page.ID] = fmt.Sprintf("Company / Strategy / Product %d", i)
	}
	write(actor, records.Pages, map[string][]string{"name": {"Unrelated notes"}, "content": {"Unrelated document."}})
	write(actor, records.Pages, map[string][]string{"name": {"Company"}, "content": {"Unselected same-name page."}})
	other, err := ws.Provision(ctx, "other@another-workspace.test")
	if err != nil {
		t.Fatal(err)
	}
	write(auth.Actor{UserID: other.ID, WorkspaceID: other.WorkspaceID}, records.Pages, map[string][]string{"name": {"Company"}, "content": {"Another workspace's private knowledge."}})
	customer := write(actor, "companies", map[string][]string{"name": {"Customer Ltd"}, "description": {"Builds scientific instruments."}, "size": {"11-50"}})
	person := write(actor, "people", map[string][]string{"name": {"Jane"}, "job_title": {"Purchasing lead"}, "company": {customer.ID}, "context": {"- Asked us about precision brackets."}})
	colleague := write(actor, "people", map[string][]string{"name": {"Alex"}, "company": {customer.ID}, "context": {"- Approves the technical specification."}})
	write(actor, "people", map[string][]string{"name": {"Unrelated employee"}, "company": {customer.ID}, "context": {"- Not involved in this conversation."}})
	deal := write(actor, "deals", map[string][]string{"name": {"Bracket order"}, "company": {customer.ID}, "value": {"15000"}})
	followUp := write(actor, records.FollowUps, map[string][]string{"name": {"Answer Jane's question"}, "person": {person.ID}, "company": {customer.ID}, "deal": {deal.ID}})
	thread, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: actor.WorkspaceID, ExternalID: "complete-thread", UserID: &owner.ID, Title: "Capabilities", At: time.Now().Add(-30 * 24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []records.Record{person, colleague} {
		handle, err := store.UpsertHandle(ctx, storage.NewHandle{WorkspaceID: actor.WorkspaceID, Kind: "email", Value: p.ID + "@customer.test", Triage: interactions.Pending})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.SetTriage(ctx, storage.Verdict{WorkspaceID: actor.WorkspaceID, ID: handle.ID, Triage: interactions.Kept, DecidedBy: new(interactions.ByUser), PersonID: &p.ID}); err != nil {
			t.Fatal(err)
		}
		if err := store.AddParticipant(ctx, thread, handle.ID, "to"); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Relink(ctx, []string{thread}); err != nil {
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
			Input []struct{ Content string }
			Tools []json.RawMessage
		}
		var input followups.Conversation
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal([]byte(req.Input[0].Content), &input); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(req.Tools) != 6 {
			t.Error("drafting must advertise the six read-only tools")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(req.Input[0].Content), &fields); err != nil {
			t.Error(err)
		}
		if _, duplicate := fields["contexts"]; duplicate {
			t.Error("person summaries must appear only in their records")
		}
		requests <- input
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp_1","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{\"follow_ups\":[],\"contexts\":[],\"skip_reason\":\"The supplied conversation needs review.\"}"}]}]}`)
	}))
	t.Cleanup(api.Close)
	client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "test", Model: "gpt-6-luna", Effort: "medium"}, log.New(io.Discard))
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
		if want, ok := wantDocuments[doc.ID]; !ok || !slices.Equal(doc.Values["content"], []string{want}) || doc.Path != wantPaths[doc.ID] {
			t.Fatalf("missing, shortened or unrelated company document: %s", doc.Name)
		}
	}
	for id, fields := range map[string]map[string][]string{
		person.ID:    {"job_title": {"Purchasing lead"}, "company": {"Customer Ltd"}, "context": {"- Asked us about precision brackets."}},
		colleague.ID: {"company": {"Customer Ltd"}, "context": {"- Approves the technical specification."}},
		customer.ID:  {"description": {"Builds scientific instruments."}, "size": {"11-50"}},
		deal.ID:      {"value": {"15000"}},
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
	if len(in.Records) != 4 || len(in.Participants) != 2 {
		t.Fatalf("include both contacts, their company and deal once, without unrelated employees: %+v", in.Records)
	}
	for _, p := range in.Participants {
		i := slices.IndexFunc(in.Records, func(r followups.Record) bool { return r.ID == p.PersonID })
		if i < 0 || p.Name == "" || in.Records[i].Name != p.Name {
			t.Fatalf("participant identity must resolve to their named record: %+v", p)
		}
	}
	if len(in.FollowUps) != 1 || in.FollowUps[0].ID != followUp.ID {
		t.Fatalf("related follow-ups must appear once: %+v", in.FollowUps)
	}
	if _, _, err := crm.Upsert(ctx, actor, records.SourceUser, records.Write{Object: records.Pages, RecordID: company.ID, Set: map[string][]string{"name": {"Our business"}, "content": {"Updated capabilities."}}}); err != nil {
		t.Fatal(err)
	}
	message("latest", "What can you offer now?", time.Now())
	in = run()
	i := slices.IndexFunc(in.Company, func(r followups.Record) bool { return r.ID == company.ID })
	if i < 0 || in.Company[i].Path != "Our business" || !slices.Equal(in.Company[i].Values["content"], []string{"Updated capabilities."}) || len(in.Messages) != 3 || len(in.Company) != len(wantDocuments) {
		t.Fatal("a later draft must read current company knowledge and the complete updated thread")
	}
	for _, doc := range in.Company {
		if doc.Path != strings.Replace(wantPaths[doc.ID], "Company", "Our business", 1) {
			t.Fatalf("renaming the selected root must update every descendant path: %s", doc.Path)
		}
	}
	if _, err := ws.Update(ctx, actor, nil, nil, new("")); err != nil {
		t.Fatal(err)
	}
	message("cleared", "Any further information?", time.Now())
	if in = run(); len(in.Company) != 0 {
		t.Fatal("clearing the knowledge root must not fall back to a same-name page")
	}
	display, err := convs.Get(ctx, actor, thread)
	if err != nil || display.Messages[0].Text != "Keep our original requirements." {
		t.Fatalf("display cleanup must remain separate from original model input: %v", err)
	}
}

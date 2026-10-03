package followups_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestDraftAgentRetrievesContextWithReadOnlyTools(t *testing.T) {
	ctx := t.Context()
	store := postgrestest.New(t)
	workspaces := workspaces.NewService(store, workspaces.Config{})
	owner, err := workspaces.Provision(ctx, "writer@company.test")
	if err != nil {
		t.Fatal(err)
	}
	other, err := workspaces.Provision(ctx, "other@private.test")
	if err != nil {
		t.Fatal(err)
	}
	actor := auth.Actor{UserID: owner.ID, WorkspaceID: owner.WorkspaceID}
	foreign := auth.Actor{UserID: other.ID, WorkspaceID: other.WorkspaceID}
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
	person := write(actor, "people", map[string][]string{"name": {"Jane"}})
	root := write(actor, records.Pages, map[string][]string{"name": {"Engineering handbook"}, "content": {"Verified machining capabilities."}})
	content := strings.Repeat("Complete production specifications. λ\n", 1000) + "Verified tolerance: 0.02 mm."
	child := write(actor, records.Pages, map[string][]string{"name": {"Bracket specification"}, "parent": {root.ID}, "content": {content}})
	secret := write(foreign, records.Pages, map[string][]string{"name": {"Secret"}, "content": {"FOREIGN SECRET"}})
	thread := func(as auth.Actor, title, body string, at time.Time, personID string) string {
		t.Helper()
		id, err := store.UpsertEmailThread(ctx, storage.EmailThread{WorkspaceID: as.WorkspaceID, ExternalID: title, UserID: &as.UserID, Title: title, At: at})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.UpsertPart(ctx, storage.NewPart{InteractionID: id, Kind: "message", ExternalID: title, Direction: "received", AuthorName: "Jane", At: &at, Content: &body}); err != nil {
			t.Fatal(err)
		}
		if personID != "" {
			if err := store.AddLink(ctx, id, personID, "user"); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	oldBody := "Previous agreement.\n\nOn Monday, Jane wrote:\n> The bracket must be aluminium."
	old := thread(actor, "Older spec", oldBody, time.Now().Add(-30*24*time.Hour), person.ID)
	current := thread(actor, "New request", "Please confirm the bracket material and tolerance.", time.Now().Add(-time.Minute), person.ID)
	private := thread(foreign, "Secret thread", "FOREIGN SECRET", time.Now().Add(-time.Hour), "")
	turn := 0
	var stable map[string]json.RawMessage
	var prefix []json.RawMessage
	var firstPage string
	call := func(id, name string, arguments any) any {
		data, err := json.Marshal(arguments)
		if err != nil {
			t.Fatal(err)
		}
		return map[string]any{"type": "function_call", "id": "fc_" + id, "call_id": id, "name": name, "arguments": string(data), "status": "completed"}
	}
	search := func(offset int) any {
		return map[string]any{"object": "pages", "query": "", "where": []any{}, "offset": offset, "limit": 1}
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		var input []json.RawMessage
		if err := json.Unmarshal(request["input"], &input); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		decode := func(raw json.RawMessage, out any) {
			t.Helper()
			if err := json.Unmarshal(raw, out); err != nil {
				t.Errorf("decode: %v", err)
			}
		}
		if len(input) < len(prefix) {
			t.Error("earlier context was discarded")
			w.WriteHeader(400)
			return
		}
		for i, want := range prefix {
			var gotValue, wantValue any
			decode(input[i], &gotValue)
			decode(want, &wantValue)
			if !reflect.DeepEqual(gotValue, wantValue) {
				t.Errorf("turn %d changed provider input item %d", turn, i)
			}
		}
		delete(request, "input")
		if turn == 0 {
			stable = request
			var key string
			decode(request["prompt_cache_key"], &key)
			if key != "crm-draft:"+actor.WorkspaceID || string(request["store"]) != "false" || string(request["truncation"]) != `"disabled"` {
				t.Error("missing workspace cache key, stateless replay or truncation protection")
			}
			var tools []struct {
				Name           string
				Strict         bool
				AllowedCallers []string `json:"allowed_callers"`
			}
			decode(request["tools"], &tools)
			names := []string{}
			for _, tool := range tools {
				names = append(names, tool.Name)
				if !tool.Strict || !slices.Equal(tool.AllowedCallers, []string{"direct"}) {
					t.Errorf("tool must use strict direct calls: %+v", tool)
				}
			}
			if !slices.Equal(names, []string{"list_objects", "search_records", "get_record", "list_interactions", "search_interactions", "get_interaction"}) {
				t.Errorf("unexpected tool access: %v", names)
			}
			var message struct{ Content string }
			decode(input[0], &message)
			var conv followups.Conversation
			decode(json.RawMessage(message.Content), &conv)
			if len(conv.Company) != 0 || len(conv.Messages) != 1 {
				t.Error("fixture must require retrieval beyond the initial conversation, without a Company root")
			}
		} else if !reflect.DeepEqual(request, stable) {
			t.Error("instructions, schemas, model, cache key or other request options changed between calls")
		}
		results := map[string]string{}
		for _, raw := range input {
			var item struct {
				Type   string
				CallID string `json:"call_id"`
				Output string
			}
			decode(raw, &item)
			if item.Type == "function_call_output" {
				results[item.CallID] = item.Output
			}
		}
		if strings.Contains(fmt.Sprint(results), "FOREIGN SECRET") {
			t.Error("another workspace's content reached the model")
		}
		var output []any
		switch turn {
		case 0:
			output = []any{map[string]any{"type": "reasoning", "id": "rs_1", "summary": []any{}, "encrypted_content": "opaque-provider-reasoning", "provider_extension": "keep-me"},
				call("objects", "list_objects", map[string]any{}), call("page0", "search_records", search(0)), call("history", "search_interactions", map[string]any{"query": "Older spec", "limit": 10})}
		case 1:
			var found struct {
				Records []followups.Record
				Total   int
			}
			decode(json.RawMessage(results["page0"]), &found)
			if found.Total != 2 || len(found.Records) != 1 || !strings.Contains(results["objects"], "pages") || !strings.Contains(results["history"], old) {
				t.Errorf("discovery failed: %+v", results)
				w.WriteHeader(400)
				return
			}
			firstPage = found.Records[0].ID
			output = []any{call("read0", "get_record", map[string]any{"record_id": firstPage}), call("page1", "search_records", search(1)), call("timeline0", "list_interactions", map[string]any{"record_id": person.ID, "cursor": "", "limit": 1})}
		case 2:
			var found struct{ Records []followups.Record }
			decode(json.RawMessage(results["page1"]), &found)
			var timeline []interactions.Interaction
			decode(json.RawMessage(results["timeline0"]), &timeline)
			if len(found.Records) != 1 || found.Records[0].ID == firstPage || len(timeline) != 1 || timeline[0].ID != current {
				t.Error("record or conversation pagination failed")
				w.WriteHeader(400)
				return
			}
			output = []any{call("read1", "get_record", map[string]any{"record_id": found.Records[0].ID}), call("old", "get_interaction", map[string]any{"interaction_id": old}), call("timeline1", "list_interactions", map[string]any{"record_id": person.ID, "cursor": current, "limit": 1}),
				call("children", "search_records", map[string]any{"object": "pages", "query": "", "where": []any{map[string]string{"attribute": "parent", "value": root.ID}}, "offset": 0, "limit": 100})}
		case 3:
			var before interactions.Interaction
			decode(json.RawMessage(results["old"]), &before)
			if len(before.Messages) != 1 || before.Messages[0].Text != oldBody || !strings.Contains(results["timeline1"], old) || !strings.Contains(results["children"], child.ID) {
				t.Error("complete prior history or reference traversal missing")
			}
			for _, id := range []string{"read0", "read1"} {
				var page followups.Record
				decode(json.RawMessage(results[id]), &page)
				if page.ID == child.ID && (!slices.Equal(page.Values["content"], []string{content}) || !slices.Equal(page.References["parent"], []string{root.ID})) {
					t.Error("page content was cut or reference IDs were lost")
				}
			}
			output = []any{
				call("write", "update_record", map[string]any{"record_id": root.ID, "content": "corrupted"}),
				call("send", "send_message", map[string]any{}),
				call("foreign", "get_record", map[string]any{"record_id": secret.ID}),
				call("foreign-thread", "get_interaction", map[string]any{"interaction_id": private}),
				call("workspace", "get_record", map[string]any{"record_id": root.ID, "workspace": foreign.WorkspaceID}),
				call("missing", "get_record", map[string]any{}),
				call("invalid", "search_records", map[string]any{"object": "pages", "query": "", "where": []any{}, "offset": 0, "limit": -1}),
				call("overflow", "search_records", search(1<<32)),
				call("negative-offset", "search_records", search(-1)),
				call("timeline-limit", "list_interactions", map[string]any{"record_id": person.ID, "cursor": "", "limit": 101}),
				call("search-limit", "search_interactions", map[string]any{"query": "", "limit": 0}),
			}
		case 4:
			for _, id := range []string{"write", "send", "foreign", "foreign-thread", "workspace", "missing", "invalid", "overflow", "negative-offset", "timeline-limit", "search-limit"} {
				var result struct{ Error string }
				decode(json.RawMessage(results[id]), &result)
				if result.Error == "" {
					t.Errorf("unsafe or invalid call %s was accepted: %s", id, results[id])
				}
			}
			answer, _ := json.Marshal(followups.Plan{FollowUps: []followups.Change{{Action: "Confirm bracket specification", Person: person.ID, Status: "Open", WaitingOn: "Us", Reply: "Hi Jane, the bracket is aluminium with a 0.02 mm tolerance."}}, Contexts: []followups.Context{}})
			output = []any{map[string]any{"type": "message", "id": "msg_final", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": string(answer), "annotations": []any{}}}}}
		default:
			t.Errorf("unexpected model turn %d", turn)
			w.WriteHeader(400)
			return
		}
		prefix = input
		for _, item := range output {
			raw, _ := json.Marshal(item)
			prefix = append(prefix, raw)
		}
		turn++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": fmt.Sprintf("resp_%d", turn), "object": "response", "status": "completed", "output": output})
	}))
	t.Cleanup(api.Close)
	logger := log.New(io.Discard)
	client := llm.New(llm.Config{BaseURL: api.URL, APIKey: "test", Model: "gpt-6-luna", Effort: "medium"}, logger)
	agent := followups.NewAgent(followups.AgentParams{Service: followups.NewService(crm, store, nil, store), Workspaces: store, Interactions: convs, Logger: logger, Planner: client})
	if n, err := agent.Run(ctx, actor.WorkspaceID); err != nil || n != 1 || turn != 5 {
		t.Fatalf("draft loop: conversations=%d model turns=%d error=%v", n, turn, err)
	}
	drafts, total, err := crm.Search(ctx, actor, records.Search{Object: records.FollowUps})
	if err != nil || total != 1 || !strings.Contains(fmt.Sprint(drafts), "0.02 mm tolerance") {
		t.Fatalf("retrieval-based draft was not saved: %+v (%v)", drafts, err)
	}
	unchanged, err := crm.Get(ctx, actor, root.ID)
	if err != nil || !strings.Contains(fmt.Sprint(unchanged.Fields), "Verified machining capabilities.") {
		t.Fatalf("denied tool altered a page: %+v (%v)", unchanged, err)
	}
}

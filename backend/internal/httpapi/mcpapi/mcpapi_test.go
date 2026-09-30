package mcpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/gluonfield/jaz-crm/backend/internal/auth"
	"github.com/gluonfield/jaz-crm/backend/internal/connections"
	"github.com/gluonfield/jaz-crm/backend/internal/httpapi/mcpapi"
	"github.com/gluonfield/jaz-crm/backend/internal/interactions"
	"github.com/gluonfield/jaz-crm/backend/internal/records"
	"github.com/gluonfield/jaz-crm/backend/internal/storage/postgres/postgrestest"
	"github.com/gluonfield/jaz-crm/backend/internal/workspaces"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearer string

func (key bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+string(key))
	return http.DefaultTransport.RoundTrip(req)
}

// connect serves /mcp and returns sessions for the owners of two workspaces.
func connect(t *testing.T) (*mcp.ClientSession, *mcp.ClientSession) {
	t.Helper()
	ctx := context.Background()
	store := postgrestest.New(t)
	keys := auth.NewService(store, auth.Config{PublicURL: "http://crm.test"})
	people := workspaces.NewService(store, workspaces.Config{})
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	conns, err := connections.NewService(store, connections.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mcpapi.NewHandler(mcpapi.Services{Records: crm, Workspaces: people, Interactions: convs, Connections: conns}, keys, log.New(io.Discard)).MCP)
	t.Cleanup(srv.Close)
	var sessions []*mcp.ClientSession
	for _, email := range []string{"a@jaz.test", "b@jaz.test"} {
		user, err := people.Provision(ctx, email)
		if err != nil {
			t.Fatal(err)
		}
		key, _, err := keys.CreateKey(ctx, user.ID, "test", "")
		if err != nil {
			t.Fatal(err)
		}
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: bearer(key)}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		sessions = append(sessions, session)
	}
	return sessions[0], sessions[1]
}

// call returns a tool's structured result, or its error text.
func call(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (map[string]any, string) {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		return nil, res.Content[0].(*mcp.TextContent).Text
	}
	var out map[string]any
	raw, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(raw, &out)
	return out, ""
}

func mustCall(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	out, failure := call(t, session, name, args)
	if failure != "" {
		t.Fatalf("%s: %s", name, failure)
	}
	return out
}

func encode(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

// Records show single values as values, multi values as lists and
// references as the record's id and name.
func TestRecordTools(t *testing.T) {
	a, _ := connect(t)
	objects := encode(mustCall(t, a, "list_objects", nil))
	if !strings.Contains(objects, `{"name":"Company","slug":"company","target":"companies","type":"reference"}`) {
		t.Fatalf("list_objects: %s", objects)
	}
	company := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme", "domains": "acme.com"}})["record"].(map[string]any)
	person := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{
		"name": "Bob", "email_addresses": []any{"bob@acme.com", "Bob@Personal.dev"}, "company": "acme.com",
	}})["record"].(map[string]any)
	values := encode(person["values"])
	want := `{"company":{"id":"` + company["id"].(string) + `","name":"Acme"},"email_addresses":["bob@acme.com","bob@personal.dev"],"name":"Bob"}`
	if values != want {
		t.Fatalf("values:\n got %s\nwant %s", values, want)
	}
	if got := encode(mustCall(t, a, "get_record", map[string]any{"record_id": person["id"]})["values"]); got != want {
		t.Fatalf("get_record: %s", got)
	}
	found := mustCall(t, a, "search_records", map[string]any{"object": "people", "where": map[string]any{"email_addresses": "BOB@personal.dev"}})["records"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["id"] != person["id"] {
		t.Fatalf("search_records: %v", found)
	}
	if _, failure := call(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": 7}}); !strings.Contains(failure, "strings") {
		t.Fatalf("a number value: %q", failure)
	}
	if _, failure := call(t, a, "upsert_record", map[string]any{"object": "deals", "values": map[string]any{"name": "x"}}); !strings.Contains(failure, "objects are companies, people") {
		t.Fatalf("an unknown object: %q", failure)
	}
}

// No tool reads, writes or references another workspace's records.
func TestTenantIsolation(t *testing.T) {
	a, b := connect(t)
	company := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme", "domains": "acme.com"}})["record"].(map[string]any)
	id := company["id"].(string)

	if _, failure := call(t, b, "get_record", map[string]any{"record_id": id}); failure == "" {
		t.Error("get_record read another workspace")
	}
	if _, failure := call(t, b, "upsert_record", map[string]any{"object": "companies", "record_id": id, "values": map[string]any{"name": "Mine"}}); failure == "" {
		t.Error("upsert_record wrote another workspace")
	}
	if _, failure := call(t, b, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Eve", "company": id}}); failure == "" {
		t.Error("upsert_record referenced another workspace")
	}
	if found := mustCall(t, b, "search_records", map[string]any{"object": "companies"})["records"].([]any); len(found) != 0 {
		t.Errorf("search_records listed another workspace: %v", found)
	}
	if found := mustCall(t, b, "search_records", map[string]any{"object": "people", "where": map[string]any{"company": id}})["records"].([]any); len(found) != 0 {
		t.Errorf("search_records filtered by another workspace's record: %v", found)
	}
	if members := encode(mustCall(t, b, "get_workspace", nil)); strings.Contains(members, "a@jaz.test") {
		t.Errorf("get_workspace listed another workspace: %s", members)
	}
	if got := mustCall(t, a, "get_record", map[string]any{"record_id": id}); encode(got["values"]) != `{"domains":["acme.com"],"name":"Acme"}` {
		t.Errorf("the owner's record changed: %v", got)
	}
}

// Conversations and triage are per workspace too, and a logged call links
// the people and records it names.
func TestInteractionTools(t *testing.T) {
	a, b := connect(t)
	company := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme", "domains": "acme.com"}})["record"].(map[string]any)
	logged := mustCall(t, a, "log_interaction", map[string]any{
		"kind": "call", "title": "Pricing", "people": []any{"Ada <ada@acme.com>"}, "records": []any{company["id"]}, "notes": "Wants 200 a week.",
	})
	id := logged["id"].(string)
	if records := encode(logged["records"]); !strings.Contains(records, `"object":"people"`) || !strings.Contains(records, `"name":"Acme"`) {
		t.Fatalf("logged records: %s", records)
	}
	timeline := mustCall(t, a, "list_interactions", map[string]any{"record_id": company["id"]})["interactions"].([]any)
	if len(timeline) != 1 || timeline[0].(map[string]any)["preview"] != "Wants 200 a week." {
		t.Fatalf("timeline: %v", timeline)
	}
	if found := mustCall(t, a, "search_interactions", map[string]any{"query": "week"})["interactions"].([]any); len(found) != 1 {
		t.Fatalf("search: %v", found)
	}
	kept := encode(mustCall(t, a, "list_triage", map[string]any{"status": "kept"}))
	if !strings.Contains(kept, `"address":"ada@acme.com"`) {
		t.Fatalf("triage: %s", kept)
	}
	record := mustCall(t, a, "get_record", map[string]any{"record_id": company["id"]})
	if activity := record["activity"].(map[string]any); activity["interactions"] != float64(1) {
		t.Fatalf("activity: %v", activity)
	}

	for tool, args := range map[string]map[string]any{
		"get_interaction":   {"interaction_id": id},
		"skip_interaction":  {"interaction_id": id},
		"link_interaction":  {"interaction_id": id, "record_id": company["id"]},
		"list_interactions": {"record_id": company["id"]},
	} {
		out, failure := call(t, b, tool, args)
		if failure == "" && tool != "list_interactions" || tool == "list_interactions" && len(out["interactions"].([]any)) != 0 {
			t.Errorf("%s reached another workspace: %v", tool, out)
		}
	}
	if other := encode(mustCall(t, b, "list_triage", map[string]any{"status": "kept"})); strings.Contains(other, "ada@acme.com") {
		t.Errorf("list_triage listed another workspace: %s", other)
	}
	if _, failure := call(t, a, "decide_triage", map[string]any{"addresses": []any{"x@y.io"}, "decision": "maybe"}); !strings.Contains(failure, "keep or skip") {
		t.Errorf("a bad decision: %q", failure)
	}
	if out := mustCall(t, a, "list_connections", nil); len(out["connections"].([]any)) != 0 || out["connect_url"] != nil {
		t.Errorf("connections without Google configured: %v", out)
	}
}

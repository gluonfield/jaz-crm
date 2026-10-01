package mcpapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gluonfield/jaz-crm/backend/internal/logos"
	"github.com/gluonfield/jaz-crm/backend/internal/storage"
	"github.com/gluonfield/jaz-tasks/auth"

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

type env struct {
	url    string
	keys   *auth.Service
	people *workspaces.Service
	store  storage.LogoStore
}

// serve runs /mcp over a fresh database.
func serve(t *testing.T) env {
	t.Helper()
	store := postgrestest.New(t)
	keys := auth.NewService(store, auth.Config{PublicURL: "http://crm.test"})
	people := workspaces.NewService(store, workspaces.Config{})
	crm := records.NewService(store)
	convs := interactions.NewService(interactions.Params{Store: store, Connections: store, Workspaces: store, Records: crm})
	conns, err := connections.NewService(store, connections.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mcpapi.NewHandler(mcpapi.Services{Records: crm, Workspaces: people, Interactions: convs, Connections: conns, Logos: logos.NewService(store, logos.Fetcher{})}, keys, log.New(io.Discard)).MCP)
	t.Cleanup(srv.Close)
	return env{url: srv.URL, keys: keys, people: people, store: store}
}

// session opens an MCP session that sends token.
func (e env) session(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: e.url, HTTPClient: &http.Client{Transport: bearer(token)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// apiKey provisions an account for email and returns its API key.
func (e env) apiKey(t *testing.T, email string) string {
	t.Helper()
	user, err := e.people.Provision(context.Background(), email)
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := e.keys.CreateKey(context.Background(), user.ID, "test", "")
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// connect returns sessions for the owners of two workspaces.
func connect(t *testing.T) (*mcp.ClientSession, *mcp.ClientSession) {
	t.Helper()
	e := serve(t)
	return e.session(t, e.apiKey(t, "a@jaz.test")), e.session(t, e.apiKey(t, "b@jaz.test"))
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
// references as the record's id, name and picture.
func TestRecordTools(t *testing.T) {
	e := serve(t)
	a := e.session(t, e.apiKey(t, "a@jaz.test"))
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
	if err := e.store.SaveLogo(context.Background(), storage.Logo{Domain: "acme.com", ContentType: "image/png", Image: []byte("png")}); err != nil {
		t.Fatal(err)
	}
	found := mustCall(t, a, "search_records", map[string]any{"object": "people", "where": map[string]any{"email_addresses": "BOB@personal.dev"}})["records"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["id"] != person["id"] {
		t.Fatalf("search_records: %v", found)
	}
	if ref := encode(found[0].(map[string]any)["values"].(map[string]any)["company"]); !strings.Contains(ref, `"photo":"http://crm.test/logos/`) {
		t.Fatalf("a referenced company's logo: %s", ref)
	}
	if _, failure := call(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": 7}}); !strings.Contains(failure, "strings") {
		t.Fatalf("a number value: %q", failure)
	}
	if _, failure := call(t, a, "upsert_record", map[string]any{"object": "quotes", "values": map[string]any{"name": "x"}}); !strings.Contains(failure, "objects are companies, people, deals") {
		t.Fatalf("an unknown object: %q", failure)
	}
}

func TestPersonContext(t *testing.T) {
	e := serve(t)
	session := e.session(t, e.apiKey(t, "context@jaz.test"))
	const original = "Introduced at Oxford.\nInterested in factory automation."
	person := mustCall(t, session, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Ada", "context": original}})["record"].(map[string]any)
	id := person["id"]
	if got := mustCall(t, session, "get_record", map[string]any{"record_id": id})["values"].(map[string]any)["context"]; got != original {
		t.Fatalf("multiline context did not persist: %v", got)
	}
	const revised = "Introduced at Oxford.\nNow exploring a pilot together."
	mustCall(t, session, "upsert_record", map[string]any{"object": "people", "record_id": id, "values": map[string]any{"context": revised}})
	found := mustCall(t, session, "search_records", map[string]any{"object": "people", "query": "pilot together"})["records"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["id"] != id || found[0].(map[string]any)["values"].(map[string]any)["context"] != revised {
		t.Fatalf("updated context is not searchable: %v", found)
	}
	history := mustCall(t, session, "record_history", map[string]any{"record_id": id})["changes"].([]any)
	var revisions []any
	for _, entry := range history {
		change := entry.(map[string]any)
		if change["attribute"] == "context" {
			revisions = append(revisions, change["value"])
		}
	}
	if encode(revisions) != encode([]string{revised, original}) {
		t.Fatalf("context history lost a revision: %v", revisions)
	}
	mustCall(t, session, "upsert_record", map[string]any{"object": "people", "record_id": id, "remove": map[string]any{"context": []string{}}})
	if got := mustCall(t, session, "get_record", map[string]any{"record_id": id})["values"].(map[string]any)["context"]; got != nil {
		t.Fatalf("context was not cleared: %v", got)
	}
}

// No tool reads, writes or references another workspace's records.
func TestTenantIsolation(t *testing.T) {
	a, b := connect(t)
	mustCall(t, a, "add_attribute_option", map[string]any{"object": "companies", "attribute": "categories", "value": "Manufacturing"})
	if objects := encode(mustCall(t, b, "list_objects", nil)); strings.Contains(objects, "Manufacturing") {
		t.Errorf("category options crossed workspaces: %s", objects)
	}
	mustCall(t, b, "add_attribute_option", map[string]any{"object": "companies", "attribute": "categories", "value": "Robotics"})
	if objects := encode(mustCall(t, a, "list_objects", nil)); strings.Contains(objects, "Robotics") {
		t.Errorf("adding a category changed another workspace: %s", objects)
	}
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
	if _, failure := call(t, b, "delete_record", map[string]any{"record_id": id}); failure == "" {
		t.Error("delete_record deleted another workspace's record")
	}
	if got := mustCall(t, a, "get_record", map[string]any{"record_id": id}); encode(got["values"]) != `{"domains":["acme.com"],"name":"Acme"}` {
		t.Errorf("the owner's record changed: %v", got)
	}
}

func TestPipelineStages(t *testing.T) {
	a, b := connect(t)
	args := map[string]any{"object": "deals", "attribute": "stage", "value": " Negotiation "}
	if out := mustCall(t, a, "add_attribute_option", args); out["value"] != "Negotiation" {
		t.Fatalf("new stage: %v", out)
	}
	args["value"] = "NEGOTIATION"
	if out := mustCall(t, a, "add_attribute_option", args); out["value"] != "Negotiation" {
		t.Fatalf("existing stage: %v", out)
	}
	if schema := encode(mustCall(t, a, "list_objects", nil)); !strings.Contains(schema, `"options":["Lead","In progress","On hold","Won","Lost","Negotiation"]`) {
		t.Fatalf("stage was not appended once: %s", schema)
	}
	if schema := encode(mustCall(t, b, "list_objects", nil)); strings.Contains(schema, "Negotiation") {
		t.Fatalf("pipeline stages crossed workspaces: %s", schema)
	}
	values := map[string]any{"name": "Acme contract", "stage": "negotiation"}
	deal := mustCall(t, a, "upsert_record", map[string]any{"object": "deals", "values": values})["record"].(map[string]any)
	if stage := deal["values"].(map[string]any)["stage"]; stage != "Negotiation" {
		t.Fatalf("deal in the new stage: %v", deal)
	}
	if _, failure := call(t, b, "upsert_record", map[string]any{"object": "deals", "values": values}); failure == "" {
		t.Fatal("another workspace used the private stage")
	}
	mustCall(t, a, "upsert_record", map[string]any{"object": "deals", "record_id": deal["id"], "values": map[string]any{"stage": "Lead"}})
	mustCall(t, a, "upsert_record", map[string]any{"object": "deals", "record_id": deal["id"], "values": map[string]any{"stage": "Negotiation"}})
	if saved := mustCall(t, a, "get_record", map[string]any{"record_id": deal["id"]}); saved["values"].(map[string]any)["stage"] != "Negotiation" {
		t.Fatalf("moving an existing deal to the new stage: %v", saved)
	}
}

func TestPipelineStageManagement(t *testing.T) {
	a, b := connect(t)
	deal := mustCall(t, a, "upsert_record", map[string]any{"object": "deals", "values": map[string]any{"name": "Contract", "stage": "Lead", "value": "1200"}})["record"].(map[string]any)
	edit := func(action, stage string, extra map[string]any) {
		t.Helper()
		args := map[string]any{"object": "deals", "attribute": "stage", "action": action, "stage": stage}
		for key, value := range extra {
			args[key] = value
		}
		mustCall(t, a, "edit_pipeline_stage", args)
	}
	assertStages := func(expected string) {
		t.Helper()
		if schema := encode(mustCall(t, a, "list_objects", nil)); !strings.Contains(schema, `"options":`+expected) {
			t.Fatalf("pipeline order: %s; wanted %s", schema, expected)
		}
	}
	edit("rename", "Lead", map[string]any{"name": " Qualified "})
	assertStages(`["Qualified","In progress","On hold","Won","Lost"]`)
	if saved := mustCall(t, a, "get_record", map[string]any{"record_id": deal["id"]}); encode(saved["values"]) != `{"name":"Contract","owner":"a@jaz.test","stage":"Qualified","value":"1200"}` {
		t.Fatalf("rename lost deal values: %v", saved)
	}
	if schema := encode(mustCall(t, b, "list_objects", nil)); strings.Contains(schema, "Qualified") {
		t.Fatalf("renaming affected another workspace: %s", schema)
	}
	for _, args := range []map[string]any{
		{"action": "rename", "stage": "Qualified", "name": "won"},
		{"action": "rename", "stage": "Qualified", "name": " "},
		{"action": "rename", "stage": "Qualified", "name": strings.Repeat("x", 81)},
		{"action": "delete", "stage": "Qualified"},
		{"action": "delete", "stage": "Qualified", "replacement": "Qualified"},
		{"action": "move", "stage": "Qualified", "before": "missing"},
	} {
		args["object"], args["attribute"] = "deals", "stage"
		if _, failure := call(t, a, "edit_pipeline_stage", args); failure == "" {
			t.Fatalf("invalid stage change accepted: %v", args)
		}
		assertStages(`["Qualified","In progress","On hold","Won","Lost"]`)
	}
	if _, failure := call(t, b, "edit_pipeline_stage", map[string]any{"object": "deals", "attribute": "stage", "action": "delete", "stage": "Qualified"}); failure == "" {
		t.Fatal("another workspace changed the private stage")
	}
	edit("move", "Won", map[string]any{"before": "Qualified"})
	assertStages(`["Won","Qualified","In progress","On hold","Lost"]`)
	created := mustCall(t, a, "upsert_record", map[string]any{"object": "deals", "values": map[string]any{"name": "New contract"}})["record"].(map[string]any)
	if created["values"].(map[string]any)["stage"] != "Won" {
		t.Fatalf("new records did not use the reordered first stage: %v", created)
	}
	edit("move", "Won", nil)
	assertStages(`["Qualified","In progress","On hold","Lost","Won"]`)
	edit("delete", "Qualified", map[string]any{"replacement": "In progress"})
	assertStages(`["In progress","On hold","Lost","Won"]`)
	if saved := mustCall(t, a, "get_record", map[string]any{"record_id": deal["id"]}); saved["values"].(map[string]any)["stage"] != "In progress" {
		t.Fatalf("deleting a stage orphaned its deal: %v", saved)
	}
	history := encode(mustCall(t, a, "record_history", map[string]any{"record_id": deal["id"]}))
	for _, value := range []string{`"value":"Lead"`, `"value":"Qualified"`, `"value":"In progress"`} {
		if !strings.Contains(history, value) {
			t.Fatalf("stage history lost %s: %s", value, history)
		}
	}
	edit("delete", "Lost", nil)
	edit("delete", "On hold", nil)
	edit("delete", "Won", map[string]any{"replacement": "In progress"})
	if _, failure := call(t, a, "edit_pipeline_stage", map[string]any{"object": "deals", "attribute": "stage", "action": "delete", "stage": "In progress"}); failure == "" {
		t.Fatal("deleted the last stage")
	}
	assertStages(`["In progress"]`)
}

func TestCompanyCategories(t *testing.T) {
	a, _ := connect(t)
	for _, value := range []string{" Manufacturing ", "B2B"} {
		mustCall(t, a, "add_attribute_option", map[string]any{"object": "companies", "attribute": "categories", "value": value})
	}
	args := map[string]any{"object": "companies", "attribute": "categories", "value": "MANUFACTURING"}
	if out := mustCall(t, a, "add_attribute_option", args); out["value"] != "Manufacturing" {
		t.Fatalf("case-insensitive option reuse: %v", out)
	}
	objects := mustCall(t, a, "list_objects", nil)["objects"].([]any)
	category := objects[0].(map[string]any)["attributes"].([]any)[1].(map[string]any)
	if got := encode(category); got != `{"multi":true,"name":"Categories","options":["Manufacturing","B2B"],"slug":"categories","type":"select"}` {
		t.Fatalf("persisted category schema: %s", got)
	}
	company := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{
		"name": "Acme", "domains": "acme.test", "categories": []any{"manufacturing", "B2B"},
	}})["record"].(map[string]any)
	id := company["id"]
	if got := encode(company["values"].(map[string]any)["categories"]); got != `["Manufacturing","B2B"]` {
		t.Fatalf("multiple canonical categories: %s", got)
	}
	search := map[string]any{"object": "companies", "where": map[string]any{"categories": "MANUFACTURING"}, "query": "Acme"}
	if found := mustCall(t, a, "search_records", search)["records"].([]any); len(found) != 1 || found[0].(map[string]any)["id"] != id {
		t.Fatalf("category filter combined with search: %v", found)
	}
	mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "record_id": id, "remove": map[string]any{"categories": []any{"Manufacturing"}}})
	if found := mustCall(t, a, "search_records", search)["records"].([]any); len(found) != 0 {
		t.Fatalf("removed category still matched: %v", found)
	}
	if got := encode(mustCall(t, a, "get_record", map[string]any{"record_id": id})["values"].(map[string]any)["categories"]); got != `["B2B"]` {
		t.Fatalf("removing one category affected another: %s", got)
	}
	if schema := encode(mustCall(t, a, "list_objects", nil)); !strings.Contains(schema, `"options":["Manufacturing","B2B"]`) {
		t.Fatalf("an unassigned category was forgotten: %s", schema)
	}
	for _, input := range []map[string]any{
		{"object": "companies", "attribute": "domains", "value": "B2B"},
		{"object": "companies", "attribute": "categories", "value": "  "},
		{"object": "companies", "attribute": "categories", "value": strings.Repeat("x", 81)},
	} {
		if _, failure := call(t, a, "add_attribute_option", input); failure == "" {
			t.Errorf("invalid option was accepted: %v", input)
		}
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
	if people := encode(mustCall(t, a, "search_records", map[string]any{"object": "people"})); !strings.Contains(people, `"interactions":1,`) {
		t.Fatalf("search_records activity: %s", people)
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

// oauth runs the OAuth flow an MCP host such as Jaz runs for user and
// returns the access token its connection sends.
func (e env) oauth(t *testing.T, user storage.User) string {
	t.Helper()
	ctx := context.Background()
	app, err := e.keys.RegisterClient(ctx, "Jaz", []string{"http://localhost/cb"})
	if err != nil {
		t.Fatal(err)
	}
	verifier := strings.Repeat("verifier", 8)
	sum := sha256.Sum256([]byte(verifier))
	target, err := e.keys.Approve(ctx, auth.Actor{UserID: user.ID, WorkspaceID: user.WorkspaceID}, auth.AuthorizeRequest{
		ResponseType: "code", ClientID: app.ID, RedirectURI: "http://localhost/cb",
		CodeChallenge: base64.RawURLEncoding.EncodeToString(sum[:]), CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	redirect, _ := url.Parse(target)
	tokens, err := e.keys.Token(ctx, auth.TokenRequest{
		GrantType: "authorization_code", ClientID: app.ID, Code: redirect.Query().Get("code"),
		RedirectURI: "http://localhost/cb", CodeVerifier: verifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tokens.AccessToken
}

// An agent names the workspace each call acts in over one MCP session, the
// way Jaz stays connected; leaving it out keeps the default, which only the
// app's workspace menu moves. An API key acts only in its own workspace.
func TestWorkspacesPerCall(t *testing.T) {
	e := serve(t)
	pat, err := e.people.SignIn(context.Background(), signin.Identity{Issuer: "https://idp.test", Subject: "pat", Email: "pat@example.com", EmailVerified: true, Name: "Pat"})
	if err != nil {
		t.Fatal(err)
	}
	jaz := e.session(t, e.oauth(t, pat))
	before := mustCall(t, jaz, "get_profile", nil)
	mustCall(t, jaz, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Home Co"}})
	created := mustCall(t, jaz, "create_workspace", map[string]any{"name": " Side project "})
	if created["name"] != "Side project" || created["default"] != nil {
		t.Fatalf("created: %v", created)
	}
	if after := mustCall(t, jaz, "get_profile", nil); after["id"] != before["id"] {
		t.Fatalf("creating moved the default workspace: before=%v after=%v", before, after)
	}
	side := map[string]any{"workspace": "side PROJECT"}
	if got := mustCall(t, jaz, "get_workspace", side); got["name"] != "Side project" || !strings.Contains(encode(got["members"]), `"admin":true`) {
		t.Fatalf("a named workspace should be the new one, with pat as admin: %v", got)
	}
	mustCall(t, jaz, "upsert_record", map[string]any{"workspace": "Side project", "object": "companies", "values": map[string]any{"name": "Side Co"}})
	companies := func(args map[string]any) string {
		found := mustCall(t, jaz, "search_records", args)["records"].([]any)
		if len(found) != 1 {
			t.Fatalf("each workspace holds one company: %v", found)
		}
		return found[0].(map[string]any)["values"].(map[string]any)["name"].(string)
	}
	if home, other := companies(map[string]any{"object": "companies"}), companies(map[string]any{"object": "companies", "workspace": "Side project"}); home != "Home Co" || other != "Side Co" {
		t.Fatalf("records crossed workspaces: %q %q", home, other)
	}
	listed := mustCall(t, jaz, "list_workspaces", nil)["workspaces"].([]any)
	if len(listed) != 2 || listed[0].(map[string]any)["default"] != true || listed[1].(map[string]any)["default"] != nil {
		t.Fatalf("workspaces: %v", listed)
	}
	if _, failure := call(t, jaz, "search_records", map[string]any{"object": "companies", "workspace": "Nope"}); !strings.Contains(failure, "Side project") {
		t.Fatalf("an unknown name should list the person's workspaces: %q", failure)
	}

	// The app's workspace menu moves the default.
	mustCall(t, jaz, "switch_workspace", map[string]any{"workspace_id": created["id"]})
	if moved := companies(map[string]any{"object": "companies"}); moved != "Side Co" {
		t.Fatalf("after the menu switch, the default is the side project: %q", moved)
	}
	mustCall(t, jaz, "switch_workspace", map[string]any{"workspace_id": listed[0].(map[string]any)["id"]})
	if restored := mustCall(t, jaz, "get_profile", nil); restored["id"] != before["id"] {
		t.Fatalf("returning to the workspace changed its profile ID: %v", restored)
	}
	if _, failure := call(t, jaz, "switch_workspace", map[string]any{"workspace_id": "00000000-0000-4000-8000-000000000000"}); !strings.Contains(failure, "not a member") {
		t.Fatalf("switch to a stranger's workspace: %q", failure)
	}

	keyed := e.session(t, e.apiKey(t, "kim@example.com"))
	mustCall(t, keyed, "create_workspace", map[string]any{"name": "Kim's other"})
	if _, failure := call(t, keyed, "get_workspace", map[string]any{"workspace": "Kim's other"}); !strings.Contains(failure, "API key") {
		t.Fatalf("an API key named another workspace: %q", failure)
	}
	if got := mustCall(t, keyed, "get_workspace", nil); got["name"] == "Kim's other" {
		t.Fatal("an API key left its workspace")
	}
}

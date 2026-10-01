package mcpapi_test

import (
	"context"
	"encoding/json"
	"html"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRecordSearchApp(t *testing.T) {
	e := serve(t)
	session := e.session(t, e.apiKey(t, "owner@example.com"))
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var resource string
	for _, tool := range tools.Tools {
		if tool.Name == "search_records" {
			resource, _ = tool.Meta["ui"].(map[string]any)["resourceUri"].(string)
		}
	}
	if resource == "" {
		t.Fatal("search_records has no interactive view")
	}
	company := mustCall(t, session, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme", "domains": []string{"acme.example"}}})["record"].(map[string]any)
	name := `Press 123 & "precision" π`
	var leadID any
	for _, stage := range []string{"Lead", "Won"} {
		record := mustCall(t, session, "upsert_record", map[string]any{"object": "deals", "values": map[string]any{"name": name, "stage": stage, "company": company["id"]}})["record"].(map[string]any)
		if stage == "Lead" {
			leadID = record["id"]
		}
	}
	where := map[string]string{"stage": "Lead", "company": "acme.example"}
	result := mustCall(t, session, "search_records", map[string]any{"object": "deals", "query": name, "where": where, "limit": 500})
	found := result["records"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["id"] != leadID {
		t.Fatalf("filtered deals: %v", found)
	}
	uri := result["resource_uri"].(string)
	path, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	var query string
	var filters map[string]string
	if err := json.Unmarshal([]byte(path.Query().Get("q")), &query); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(path.Query().Get("where")), &filters); err != nil {
		t.Fatal(err)
	}
	if path.Scheme != "ui" || path.Host != "jaz-crm" || path.Path != "/o/deals" || query != name || filters["stage"] != "Lead" || filters["company"] != "acme.example" || path.Query().Get("limit") != "100" {
		t.Fatalf("the app opens a different search: %s", path)
	}
	if _, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: resource}); err != nil {
		t.Fatal(err)
	}
	templates, err := session.ListResourceTemplates(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var searchTemplate bool
	for _, template := range templates.ResourceTemplates {
		searchTemplate = searchTemplate || template.URITemplate == "ui://jaz-crm/o/{object}{?q,where,limit,view}"
	}
	if !searchTemplate {
		t.Fatal("missing search resource template")
	}
	app, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatal(err)
	}
	if len(app.Contents) != 1 || app.Contents[0].URI != uri || !strings.Contains(app.Contents[0].Text, `data-start-path="`+html.EscapeString(path.RequestURI())+`"`) {
		t.Fatal("the filtered resource did not preserve its starting route")
	}
	opened := mustCall(t, session, "show_crm", map[string]any{"path": uri})
	if opened["path"] != uri {
		t.Fatal("show_crm lost the resource URL")
	}
}

func TestRecordCardApp(t *testing.T) {
	e := serve(t)
	session := e.session(t, e.apiKey(t, "owner@example.com"))
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var resource string
	for _, tool := range tools.Tools {
		if tool.Name == "get_record" {
			resource, _ = tool.Meta["ui"].(map[string]any)["resourceUri"].(string)
			if tool.Meta["ui/resourceUri"] != resource {
				t.Fatal("lookup resource metadata differs between hosts")
			}
		}
	}
	if resource == "" {
		t.Fatal("get_record has no interactive view")
	}
	for _, object := range []string{"people", "companies", "deals"} {
		record := mustCall(t, session, "upsert_record", map[string]any{"object": object, "values": map[string]any{"name": "Test " + object}})["record"].(map[string]any)
		result := mustCall(t, session, "get_record", map[string]any{"record_id": record["id"]})
		if result["id"] != record["id"] || result["object"] != object || encode(result["values"]) != encode(record["values"]) {
			t.Fatalf("lookup changed the record shape: %v", result)
		}
		uri, _ := result["resource_uri"].(string)
		if uri != "ui://jaz-crm/r/"+record["id"].(string) {
			t.Fatalf("lookup opens a different record: %s", uri)
		}
		app, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			t.Fatal(err)
		}
		if len(app.Contents) != 1 || app.Contents[0].URI != uri || !strings.Contains(app.Contents[0].Text, `data-start-path="/r/`+record["id"].(string)+`"`) {
			t.Fatal("record resource did not preserve its starting route")
		}
		if opened := mustCall(t, session, "show_crm", map[string]any{"path": uri}); opened["path"] != uri {
			t.Fatal("show_crm lost the record resource URL")
		}
	}
}

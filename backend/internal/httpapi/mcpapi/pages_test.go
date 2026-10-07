package mcpapi_test

import (
	"strings"
	"testing"
)

// An agent builds a table of painpoints with company references and nested
// pages that mention records, then reads and reshapes them.
func TestAgentPagesAndTables(t *testing.T) {
	a, _ := connect(t)
	mustCall(t, a, "create_object", map[string]any{"slug": "painpoints", "name": "Painpoints"})
	mustCall(t, a, "create_attribute", map[string]any{"object": "painpoints", "slug": "company", "name": "Company", "type": "reference", "target": "companies"})
	acme := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme", "domains": "acme.com"}})["record"].(map[string]any)["id"].(string)
	body := "Quotes take a week at [Acme](/r/" + acme + ")."
	pain := mustCall(t, a, "upsert_record", map[string]any{"object": "painpoints", "values": map[string]any{"name": "Slow quotes", "company": "acme.com", "content": body}})["record"].(map[string]any)["id"].(string)
	research := mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Research", "icon": "📚"}})["record"].(map[string]any)["id"].(string)
	child := mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Interviews", "parent": research, "icon": "👩🏽‍💻"}})["record"].(map[string]any)["id"].(string)

	record := mustCall(t, a, "get_record", map[string]any{"record_id": pain})
	if record["values"].(map[string]any)["content"] != body {
		t.Fatalf("get_record content: %v", record["values"])
	}
	company := mustCall(t, a, "get_record", map[string]any{"record_id": acme})
	if refs, _ := company["related"].(map[string]any)["painpoints.company"].([]any); len(refs) != 1 {
		t.Fatalf("a company lists the painpoints referencing it: %v", company["related"])
	}
	page := mustCall(t, a, "get_record", map[string]any{"record_id": research})
	if page["values"].(map[string]any)["icon"] != "📚" {
		t.Fatalf("page icon: %v", page["values"])
	}
	if refs, _ := page["related"].(map[string]any)["pages.parent"].([]any); len(refs) != 1 || refs[0].(map[string]any)["id"] != child || refs[0].(map[string]any)["icon"] != "👩🏽‍💻" {
		t.Fatalf("a page lists its sub-pages: %v", page["related"])
	}
	for _, icon := range []string{"📚", "icon:rocket", ""} {
		if icon == "icon:rocket" {
			mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "record_id": research, "values": map[string]any{"icon": icon}})
		} else if icon == "" {
			mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "record_id": research, "remove": map[string]any{"icon": []string{}}})
		}
		got := mustCall(t, a, "get_record", map[string]any{"record_id": child})
		parent := got["values"].(map[string]any)["parent"].(map[string]any)
		if value, _ := parent["icon"].(string); value != icon || parent["name"] != "Research" {
			t.Fatalf("parent icon %q: %v", icon, parent)
		}
	}
	found := mustCall(t, a, "search_records", map[string]any{"object": "painpoints", "query": "a week"})["records"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["values"].(map[string]any)["content"] != nil {
		t.Fatalf("search matches content without listing it: %v", found)
	}
	if _, failure := call(t, a, "upsert_record", map[string]any{"object": "painpoints", "record_id": pain, "values": map[string]any{"content": "Rewritten"}, "expect": map[string]any{"content": "Something older"}}); !strings.Contains(failure, "changed since it was read") {
		t.Fatalf("a stale write: %q", failure)
	}

	objects := mustCall(t, a, "list_objects", map[string]any{})["objects"].([]any)
	for _, raw := range objects {
		o := raw.(map[string]any)
		if standard, _ := o["standard"].(bool); standard != (o["slug"] != "painpoints") {
			t.Errorf("%s standard: %v", o["slug"], o["standard"])
		}
	}
	mustCall(t, a, "edit_attribute", map[string]any{"object": "painpoints", "attribute": "company", "action": "rename", "name": "Customer"})
	mustCall(t, a, "edit_object", map[string]any{"object": "painpoints", "action": "delete"})
	if _, failure := call(t, a, "get_record", map[string]any{"record_id": pain}); failure == "" {
		t.Fatal("a deleted table's record remains")
	}
}

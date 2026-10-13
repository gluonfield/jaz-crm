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
		if value, _ := parent["icon"].(string); value != icon || parent["name"] != "Research" || parent["id"] != research {
			t.Fatalf("parent icon %q: %v", icon, parent)
		}
	}
	blank := mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "values": map[string]any{"icon": "📚"}})["record"].(map[string]any)["id"].(string)
	nested := mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Untitled child", "parent": blank}})["record"].(map[string]any)["id"].(string)
	mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "record_id": blank, "remove": map[string]any{"icon": []string{}}})
	untitled, ok := mustCall(t, a, "get_record", map[string]any{"record_id": nested})["values"].(map[string]any)["parent"].(map[string]any)
	if !ok || untitled["id"] != blank || untitled["name"] != nil || untitled["icon"] != nil {
		t.Fatalf("an undecorated nameless page retains its reference: %v", untitled)
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

func TestTablesInsidePages(t *testing.T) {
	a, b := connect(t)
	root := mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Research"}})["record"].(map[string]any)["id"].(string)
	child := mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Interviews", "parent": root}})["record"].(map[string]any)["id"].(string)
	foreign := mustCall(t, b, "upsert_record", map[string]any{"object": "pages", "values": map[string]any{"name": "Private"}})["record"].(map[string]any)["id"].(string)
	company := mustCall(t, a, "upsert_record", map[string]any{"object": "companies", "values": map[string]any{"name": "Acme"}})["record"].(map[string]any)["id"].(string)
	table := mustCall(t, a, "create_object", map[string]any{"slug": "suppliers", "name": "Suppliers", "parent": child})
	if table["parent"] != child {
		t.Fatalf("new table parent: %v", table)
	}
	for _, icon := range []string{"📚", "icon:Phone:green", "image:https://example.com/logo.png", ""} {
		mustCall(t, a, "edit_object", map[string]any{"object": "suppliers", "action": "icon", "icon": icon})
		for _, raw := range mustCall(t, a, "list_objects", map[string]any{})["objects"].([]any) {
			object := raw.(map[string]any)
			if object["slug"] == "suppliers" {
				got, _ := object["icon"].(string)
				if got != icon || object["parent"] != child {
					t.Fatalf("saved database icon: %v", object)
				}
			}
		}
	}
	for _, icon := range []string{"image:javascript:alert(1)", "image:file:///private/logo.png", "data:image/png;base64,aGVsbG8=", "image:https://user:password@example.com/logo.png"} {
		if _, failure := call(t, a, "edit_object", map[string]any{"object": "suppliers", "action": "icon", "icon": icon}); failure == "" {
			t.Fatalf("unsafe database icon accepted: %s", icon)
		}
	}
	mustCall(t, a, "edit_object", map[string]any{"object": "suppliers", "action": "describe", "description": "Supplier research and quotations"})
	row := mustCall(t, a, "upsert_record", map[string]any{"object": "suppliers", "values": map[string]any{"name": "First supplier"}})["record"].(map[string]any)["id"].(string)
	body := "See [Suppliers](/o/suppliers)."
	mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "record_id": root, "values": map[string]any{"content": body}})
	if got := mustCall(t, a, "get_record", map[string]any{"record_id": root})["values"].(map[string]any)["content"]; got != body {
		t.Fatalf("table link content: %v", got)
	}
	parent := func() any {
		t.Helper()
		for _, raw := range mustCall(t, a, "list_objects", map[string]any{})["objects"].([]any) {
			object := raw.(map[string]any)
			if object["slug"] == "suppliers" {
				return object["parent"]
			}
		}
		t.Fatal("table disappeared")
		return nil
	}
	for _, invalid := range []string{foreign, company, "missing"} {
		if _, failure := call(t, a, "create_object", map[string]any{"slug": "invalid", "name": "Invalid", "parent": invalid}); failure == "" {
			t.Fatalf("created table under invalid parent %s", invalid)
		}
		if _, failure := call(t, a, "edit_object", map[string]any{"object": "suppliers", "action": "move", "parent": invalid}); failure == "" || parent() != child {
			t.Fatalf("invalid move changed nesting: %q", failure)
		}
	}
	if _, failure := call(t, b, "edit_object", map[string]any{"object": "suppliers", "action": "move", "parent": foreign}); failure == "" {
		t.Fatal("another workspace moved the table")
	}
	if _, failure := call(t, a, "edit_object", map[string]any{"object": "people", "action": "move", "parent": root}); failure == "" {
		t.Fatal("a built-in CRM table was moved")
	}
	for _, destination := range []string{root, "", child} {
		mustCall(t, a, "edit_object", map[string]any{"object": "suppliers", "action": "move", "parent": destination})
		got, _ := parent().(string)
		if got != destination {
			t.Fatalf("table parent: got %q, want %q", got, destination)
		}
	}
	for _, raw := range mustCall(t, a, "list_objects", map[string]any{})["objects"].([]any) {
		object := raw.(map[string]any)
		if object["slug"] == "suppliers" && object["description"] != "Supplier research and quotations" {
			t.Fatalf("moving changed database description: %v", object)
		}
	}
	mustCall(t, a, "delete_record", map[string]any{"record_id": child})
	if parent() != child {
		t.Fatal("Trash discarded table nesting")
	}
	if _, failure := call(t, a, "create_object", map[string]any{"slug": "trashed", "name": "Trashed", "parent": child}); failure == "" {
		t.Fatal("a table was created inside a trashed page")
	}
	mustCall(t, a, "restore_record", map[string]any{"record_id": child})
	if parent() != child || mustCall(t, a, "get_record", map[string]any{"record_id": row})["values"].(map[string]any)["name"] != "First supplier" {
		t.Fatal("restoring the page changed its table or rows")
	}
}

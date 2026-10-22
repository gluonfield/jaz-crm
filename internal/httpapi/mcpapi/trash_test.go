package mcpapi_test

import (
	"reflect"
	"testing"
)

func TestTrashPreservesDocumentsAndReferences(t *testing.T) {
	a, b := connect(t)
	page := func(name, parent string) string {
		t.Helper()
		values := map[string]any{"name": name}
		if parent != "" {
			values["parent"] = parent
		}
		return mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "values": values})["record"].(map[string]any)["id"].(string)
	}
	company := page("Company", "")
	videos := page("Videos", company)
	child := page("CNC demo", videos)
	body := "## Videos\n\n[Demo](https://example.com/video)\n\n---\n\n- [x] Reviewed"
	mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "record_id": videos, "values": map[string]any{"content": "Original"}})
	mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "record_id": videos, "values": map[string]any{"content": body, "icon": "icon:Video:blue"}})
	before := mustCall(t, a, "get_record", map[string]any{"record_id": videos})
	history := mustCall(t, a, "record_history", map[string]any{"record_id": videos})
	mustCall(t, a, "delete_record", map[string]any{"record_id": videos})
	trash := mustCall(t, a, "list_trash", nil)["records"].([]any)
	if len(trash) != 1 {
		t.Fatalf("Trash: %v", trash)
	}
	entry := trash[0].(map[string]any)
	if entry["id"] != videos || entry["name"] != "Videos" || entry["object"] != "pages" || entry["icon"] != "icon:Video:blue" || entry["deleted_at"] == nil {
		t.Fatalf("Trash entry: %v", entry)
	}
	if other := mustCall(t, b, "list_trash", nil)["records"].([]any); len(other) != 0 {
		t.Fatalf("another workspace sees Trash: %v", other)
	}
	if _, failure := call(t, b, "restore_record", map[string]any{"record_id": videos}); failure == "" {
		t.Fatal("another workspace restored the page")
	}
	for _, tool := range []string{"get_record", "upsert_record", "delete_record"} {
		if _, failure := call(t, a, tool, map[string]any{"record_id": videos, "object": "pages", "values": map[string]any{"content": "Overwritten"}}); failure == "" {
			t.Fatalf("%s reached a page in Trash", tool)
		}
	}
	if found := mustCall(t, a, "search_records", map[string]any{"object": "pages", "query": "Reviewed"})["records"].([]any); len(found) != 0 {
		t.Fatalf("Trash is searchable as active content: %v", found)
	}
	if parent := mustCall(t, a, "get_record", map[string]any{"record_id": child})["values"].(map[string]any)["parent"]; parent != nil {
		t.Fatalf("a child displays its trashed parent: %v", parent)
	}
	if _, failure := call(t, a, "upsert_record", map[string]any{"object": "pages", "record_id": company, "values": map[string]any{"parent": child}}); failure == "" {
		t.Fatal("reparenting through a trashed ancestor introduced a cycle")
	}
	mustCall(t, a, "restore_record", map[string]any{"record_id": videos})
	after := mustCall(t, a, "get_record", map[string]any{"record_id": videos})
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(history, mustCall(t, a, "record_history", map[string]any{"record_id": videos})) {
		t.Fatalf("restore changed the record or its history: before=%v after=%v", before, after)
	}
	if parent := mustCall(t, a, "get_record", map[string]any{"record_id": child})["values"].(map[string]any)["parent"].(map[string]any); parent["id"] != videos {
		t.Fatalf("restore lost the child reference: %v", parent)
	}
	if trash := mustCall(t, a, "list_trash", nil)["records"].([]any); len(trash) != 0 {
		t.Fatalf("restored record remains in Trash: %v", trash)
	}
	mustCall(t, a, "delete_record", map[string]any{"record_id": videos})
	mustCall(t, a, "upsert_record", map[string]any{"object": "pages", "record_id": child, "values": map[string]any{"parent": company}})
	mustCall(t, a, "restore_record", map[string]any{"record_id": videos})
	if parent := mustCall(t, a, "get_record", map[string]any{"record_id": child})["values"].(map[string]any)["parent"].(map[string]any); parent["id"] != company {
		t.Fatalf("restore overwrote a newer move: %v", parent)
	}
}

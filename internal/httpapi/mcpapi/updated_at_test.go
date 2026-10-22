package mcpapi_test

import (
	"net/url"
	"testing"
	"time"
)

func TestRecordUpdatedAtTools(t *testing.T) {
	a, _ := connect(t)
	first := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "First"}})["record"].(map[string]any)
	second := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Second"}})["record"].(map[string]any)
	changed := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "record_id": first["id"], "values": map[string]any{"job_title": "Engineer"}})["record"].(map[string]any)
	oldAt, err := time.Parse(time.RFC3339Nano, first["updated_at"].(string))
	if err != nil {
		t.Fatal(err)
	}
	newAt, err := time.Parse(time.RFC3339Nano, changed["updated_at"].(string))
	if err != nil || !newAt.After(oldAt) {
		t.Fatalf("tool timestamp did not advance: %v %v", changed, err)
	}
	got := mustCall(t, a, "get_record", map[string]any{"record_id": first["id"]})
	if got["updated_at"] != changed["updated_at"] || got["created_at"] != first["created_at"] {
		t.Fatalf("get and write timestamps differ: %v", got)
	}
	result := mustCall(t, a, "search_records", map[string]any{"object": "people", "sort": "updated_at"})
	rows := result["records"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["id"] != first["id"] || rows[1].(map[string]any)["id"] != second["id"] || rows[0].(map[string]any)["updated_at"] != changed["updated_at"] {
		t.Fatalf("search lost order or timestamps: %v", result)
	}
	resource, err := url.Parse(result["resource_uri"].(string))
	if err != nil || resource.Query().Get("sort") != "updated_at" {
		t.Fatalf("embedded search link lost the sort: %v %v", resource, err)
	}
}

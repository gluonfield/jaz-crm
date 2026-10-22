package mcpapi_test

import (
	"testing"
	"time"
)

func TestMessageTimelinePaging(t *testing.T) {
	a, b := connect(t)
	record := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Ada"}})["record"].(map[string]any)
	ids := map[string]bool{}
	log := func(at string, kind string) {
		t.Helper()
		entry := map[string]any{"kind": kind, "at": at, "text": "Original text", "records": []any{record["id"]}}
		if kind == "message" {
			entry["channel"] = "linkedin"
			entry["sender"] = "Ada"
			entry["recipients"] = []any{"August"}
		}
		logged := mustCall(t, a, "log_interaction", entry)
		ids[logged["id"].(string)] = true
	}
	for range 5 {
		log("2026-09-20", "message")
	}
	log("2026-09-19", "message")
	activity := mustCall(t, a, "get_record", map[string]any{"record_id": record["id"]})["activity"].(map[string]any)
	if activity["first_at"] != "2026-09-19" || activity["last_at"] != "2026-09-20" {
		t.Fatalf("date-only contact dates became timestamps: %v", activity)
	}
	log("2026-09-20T12:00:00Z", "call")
	activity = mustCall(t, a, "get_record", map[string]any{"record_id": record["id"]})["activity"].(map[string]any)
	if activity["first_at"] != "2026-09-19" || activity["last_at"] != "2026-09-20T12:00:00Z" {
		t.Fatalf("contact dates lost their individual precision: %v", activity)
	}
	seen := map[string]bool{}
	cursor := ""
	for range len(ids) + 1 {
		page := mustCall(t, a, "list_interactions", map[string]any{"record_id": record["id"], "cursor": cursor, "limit": 2})["interactions"].([]any)
		if len(page) == 0 {
			break
		}
		for _, item := range page {
			id := item.(map[string]any)["id"].(string)
			if !ids[id] || seen[id] {
				t.Fatalf("paging repeated or invented an interaction: %s", id)
			}
			seen[id] = true
			cursor = id
		}
	}
	if len(seen) != len(ids) {
		t.Fatalf("paging skipped tied dates: read %d of %d", len(seen), len(ids))
	}
	if page := mustCall(t, b, "list_interactions", map[string]any{"record_id": record["id"], "cursor": cursor})["interactions"].([]any); len(page) != 0 {
		t.Fatalf("cursor crossed workspace boundary: %v", page)
	}
	future := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano)
	log(future, "meeting")
	log(future, "meeting")
	first := mustCall(t, a, "list_interactions", map[string]any{"record_id": record["id"], "upcoming": true, "limit": 1})["interactions"].([]any)
	if len(first) != 1 {
		t.Fatalf("upcoming first page: %v", first)
	}
	second := mustCall(t, a, "list_interactions", map[string]any{"record_id": record["id"], "upcoming": true, "cursor": first[0].(map[string]any)["id"], "limit": 1})["interactions"].([]any)
	if len(second) != 1 || first[0].(map[string]any)["id"] == second[0].(map[string]any)["id"] {
		t.Fatalf("upcoming paging skipped or repeated tied meetings: %v / %v", first, second)
	}
}

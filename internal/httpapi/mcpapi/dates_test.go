package mcpapi_test

import "testing"

func TestExplicitActionDateOverride(t *testing.T) {
	owner, other := connect(t)
	r := mustCall(t, owner, "upsert_record", map[string]any{"object": "follow_ups", "values": map[string]any{"name": "Send documents"}})["record"].(map[string]any)
	id := r["id"]
	for _, date := range []string{"2026-10-04T19:00:00+01:00", ""} {
		mustCall(t, owner, "save_action_date", map[string]any{"record_id": id, "value": date})
		blocked := mustCall(t, owner, "upsert_record", map[string]any{"object": "follow_ups", "record_id": id, "values": map[string]any{"action_date": "2026-10-05", "action_date_basis": "Suggested", "action_date_reason": "An automatic review"}})
		if len(blocked["skipped"].([]any)) != 3 {
			t.Fatalf("protected date changes must report their skipped fields: %v", blocked)
		}
		got := mustCall(t, owner, "get_record", map[string]any{"record_id": id})["values"].(map[string]any)
		if got["action_date_basis"] != "Manual" || date == "" && got["action_date"] != nil || date != "" && got["action_date"] != "2026-10-04T18:00:00Z" {
			t.Fatalf("automatic writes must preserve explicit dates and clears: %v", got)
		}
	}
	if _, failure := call(t, other, "save_action_date", map[string]any{"record_id": id, "value": "2026-10-07"}); failure == "" {
		t.Fatal("manual date override crossed workspace boundary")
	}
	if _, failure := call(t, owner, "save_action_date", map[string]any{"record_id": "", "value": "2026-10-07"}); failure == "" {
		t.Fatal("date edit created a nameless record")
	}
}

package mcpapi_test

import (
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func TestDraftingStateOnConversationAndTimeline(t *testing.T) {
	e := serve(t)
	a := e.session(t, e.apiKey(t, "a@jaz.test"))
	b := e.session(t, e.apiKey(t, "b@jaz.test"))
	person := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Jane"}})["record"].(map[string]any)
	latest := time.Now().UTC().Truncate(time.Microsecond)
	conversation := mustCall(t, a, "log_interaction", map[string]any{"kind": "message", "channel": "linkedin", "at": latest.Format(time.RFC3339Nano), "sender": "Jane", "recipients": []string{"August"}, "text": "Does this work with CNC?", "records": []any{person["id"]}})
	id := conversation["id"].(string)
	store := e.store.(storage.InteractionStore)
	started, err := store.ClaimFollowUp(t.Context(), id, nil, &latest)
	if err != nil || started == nil {
		t.Fatalf("claim: %v %v", started, err)
	}
	check := func(state, reason string) {
		t.Helper()
		full := mustCall(t, a, "get_interaction", map[string]any{"interaction_id": id})
		list := mustCall(t, a, "list_interactions", map[string]any{"record_id": person["id"]})["interactions"].([]any)
		for _, view := range []map[string]any{full, list[0].(map[string]any)} {
			drafting, ok := view["drafting"].(map[string]any)
			if !ok || drafting["state"] != state || drafting["started_at"] == nil || reason != "" && drafting["reason"] != reason {
				t.Fatalf("drafting state was lost on the wire: %v", view)
			}
		}
	}
	check("drafting", "")
	reason := "CNC compatibility has not been confirmed."
	if finished, err := store.FinishFollowUp(t.Context(), id, *started, &latest, "skipped", reason); err != nil || !finished {
		t.Fatalf("finish: %v %v", finished, err)
	}
	check("skipped", reason)
	if _, failure := call(t, b, "get_interaction", map[string]any{"interaction_id": id}); failure == "" {
		t.Fatal("drafting result crossed the workspace boundary")
	}
}

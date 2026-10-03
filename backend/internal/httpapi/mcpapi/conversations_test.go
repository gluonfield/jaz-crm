package mcpapi_test

import (
	"context"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func TestConversationSearchTool(t *testing.T) {
	e := serve(t)
	a := e.session(t, e.apiKey(t, "conversations@jaz.test"))
	workspace := mustCall(t, a, "get_workspace", nil)["id"].(string)
	store := e.store.(storage.InteractionStore)
	thread, err := store.UpsertInteraction(context.Background(), storage.NewInteraction{WorkspaceID: workspace, Kind: "email", Source: "manual", ExternalID: "thread", StartedAt: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, status := range []string{"Done", "Open"} {
		r := mustCall(t, a, "upsert_record", map[string]any{"object": "follow_ups", "values": map[string]any{"name": status + " action", "status": status}})["record"].(map[string]any)
		id := r["id"].(string)
		ids = append(ids, id)
		if err := store.AddLink(context.Background(), thread.ID, id, "user"); err != nil {
			t.Fatal(err)
		}
	}
	q := map[string]any{"object": "follow_ups", "query": "Done action", "group_by_conversation": true}
	result := mustCall(t, a, "search_records", q)
	rows := result["records"].([]any)
	if result["total"] != float64(1) || len(rows) != 1 {
		t.Fatalf("conversation search count: %v", result)
	}
	row := rows[0].(map[string]any)
	if row["id"] != ids[1] || row["conversation_id"] != thread.ID {
		t.Fatalf("conversation must return the current action and exact thread: %v", row)
	}
	result = mustCall(t, a, "search_records", map[string]any{"object": "follow_ups", "conversation_id": thread.ID})
	rows = result["records"].([]any)
	if result["total"] != float64(2) || len(rows) != 2 {
		t.Fatalf("conversation history lost actions: %v", result)
	}
	for _, raw := range rows {
		if raw.(map[string]any)["conversation_id"] != thread.ID {
			t.Fatalf("history action lost conversation identity: %v", raw)
		}
	}
	for _, q := range []map[string]any{
		{"object": "people", "group_by_conversation": true},
		{"object": "people", "conversation_id": thread.ID},
		{"object": "follow_ups", "conversation_id": "invalid"},
	} {
		if _, failure := call(t, a, "search_records", q); failure == "" {
			t.Fatalf("invalid conversation query accepted: %v", q)
		}
	}
}

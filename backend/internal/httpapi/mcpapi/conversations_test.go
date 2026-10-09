package mcpapi_test

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/jaz-crm/backend/internal/storage"
)

func TestConversationSearchTool(t *testing.T) {
	e := serve(t)
	a := e.session(t, e.apiKey(t, "conversations@jaz.test"))
	workspace := mustCall(t, a, "get_workspace", nil)["id"].(string)
	store := e.store.(storage.InteractionStore)
	thread, err := store.UpsertInteraction(context.Background(), storage.NewInteraction{WorkspaceID: workspace, Kind: "message", Channel: "email", ExternalID: "thread", StartedAt: time.Now().Add(-time.Hour)})
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
	mustCall(t, a, "upsert_record", map[string]any{"object": "follow_ups", "values": map[string]any{"name": "Another conversation", "status": "Open"}})
	replayURL := func(result map[string]any, grouped bool, conversationID string) {
		t.Helper()
		uri, err := url.Parse(result["resource_uri"].(string))
		if err != nil {
			t.Fatal(err)
		}
		params := uri.Query()
		if params.Get("conversation_id") != conversationID || grouped && params.Get("view") == "table" || !grouped && params.Get("view") != "table" {
			t.Fatalf("resource lost its conversation view: %s", uri)
		}
		args := map[string]any{"object": strings.TrimPrefix(uri.Path, "/o/")}
		for _, field := range []string{"group_by_conversation", "filters"} {
			var value any
			if err := json.Unmarshal([]byte(params.Get(field)), &value); err != nil {
				t.Fatalf("resource lost %s: %s: %v", field, uri, err)
			}
			args[field] = value
		}
		if args["group_by_conversation"] != grouped || !reflect.DeepEqual(args["filters"], []any{}) {
			t.Fatalf("resource added filtering or lost grouping: %s", uri)
		}
		if query := params.Get("q"); query != "" {
			var argsQuery string
			if err := json.Unmarshal([]byte(query), &argsQuery); err != nil {
				t.Fatal(err)
			}
			args["query"] = argsQuery
		}
		args["conversation_id"] = params.Get("conversation_id")
		again := mustCall(t, a, "search_records", args)
		if !reflect.DeepEqual(again["records"], result["records"]) || again["total"] != result["total"] {
			t.Fatalf("resource changes the search on reload: %v; want %v", again, result)
		}
	}
	q := map[string]any{"object": "follow_ups", "query": "Done action", "group_by_conversation": true}
	result := mustCall(t, a, "search_records", q)
	replayURL(result, true, "")
	rows := result["records"].([]any)
	if result["total"] != float64(1) || len(rows) != 1 {
		t.Fatalf("conversation search count: %v", result)
	}
	row := rows[0].(map[string]any)
	if row["id"] != ids[1] || row["conversation_id"] != thread.ID {
		t.Fatalf("conversation must return the current action and exact thread: %v", row)
	}
	result = mustCall(t, a, "search_records", map[string]any{"object": "follow_ups", "conversation_id": thread.ID})
	replayURL(result, false, thread.ID)
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

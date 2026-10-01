package mcpapi_test

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDefaultDealFollowups(t *testing.T) {
	e := serve(t)
	key := e.apiKey(t, "a@jaz.test")
	a := e.session(t, key)
	b := e.session(t, e.apiKey(t, "b@jaz.test"))
	listed := mustCall(t, a, "list_saved_filters", map[string]any{"object": "deals"})["filters"].([]any)
	if len(listed) != 1 {
		t.Fatalf("default follow-up filter: %v", listed)
	}
	filter := listed[0].(map[string]any)
	day := time.Now().UTC()
	var want []string
	for _, fixture := range []struct {
		name, stage string
		days        int
		dated, due  bool
	}{
		{"Today", "On hold", 0, true, true},
		{"Overdue", "In progress", -1, true, true},
		{"Tomorrow", "On hold", 1, true, false},
		{"No date", "On hold", 0, false, false},
		{"Won", "Won", -1, true, false},
		{"Lost", "Lost", -1, true, false},
	} {
		values := map[string]any{"name": fixture.name, "stage": fixture.stage, "next_action": "Call customer"}
		if fixture.dated {
			values["next_follow_up_date"] = day.AddDate(0, 0, fixture.days).Format(time.DateOnly)
		}
		record := mustCall(t, a, "upsert_record", map[string]any{"object": "deals", "values": values})["record"].(map[string]any)
		if fixture.due {
			want = append(want, record["id"].(string))
		}
	}
	mustCall(t, b, "upsert_record", map[string]any{"object": "deals", "values": map[string]any{"name": "Other workspace", "next_follow_up_date": day.Format(time.DateOnly)}})
	search := map[string]any{"object": "deals", "filters": filter["filters"]}
	found := mustCall(t, a, "search_records", search)["records"].([]any)
	var ids []string
	for _, record := range found {
		ids = append(ids, record.(map[string]any)["id"].(string))
	}
	slices.Sort(ids)
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Fatalf("due follow-ups: got %v, want %v", ids, want)
	}
	mustCall(t, a, "upsert_record", map[string]any{"object": "deals", "record_id": want[0], "values": map[string]any{"next_follow_up_date": day.AddDate(0, 0, 2).Format(time.DateOnly)}})
	if got := mustCall(t, a, "search_records", search)["records"].([]any); len(got) != 1 || got[0].(map[string]any)["id"] != want[1] {
		t.Fatalf("rescheduled follow-up still matched its old date: %v", got)
	}
	updated := mustCall(t, a, "save_filter", map[string]any{"object": "deals", "id": filter["id"], "name": "My follow-ups", "filters": filter["filters"]})
	if updated["id"] != filter["id"] || updated["name"] != "My follow-ups" || !reflect.DeepEqual(updated["filters"], filter["filters"]) {
		t.Fatalf("default was not editable as an ordinary filter: %v", updated)
	}
	mustCall(t, a, "delete_saved_filter", map[string]any{"id": filter["id"]})
	if got := mustCall(t, e.session(t, key), "list_saved_filters", map[string]any{"object": "deals"})["filters"].([]any); len(got) != 0 {
		t.Fatalf("deleted default returned on reconnect: %v", got)
	}
	if got := mustCall(t, b, "list_saved_filters", map[string]any{"object": "deals"})["filters"].([]any); len(got) != 1 || got[0].(map[string]any)["id"] == filter["id"] {
		t.Fatalf("default filter identities or deletion crossed workspaces: %v", got)
	}
}

func TestSavedPeopleFiltersRoundTrip(t *testing.T) {
	e := serve(t)
	key := e.apiKey(t, "a@jaz.test")
	a := e.session(t, key)
	b := e.session(t, e.apiKey(t, "b@jaz.test"))
	for _, tag := range []string{"Founder", "Manufacturing"} {
		mustCall(t, a, "add_attribute_option", map[string]any{"object": "people", "attribute": "tags", "value": tag})
	}
	person := mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Ada Stone", "tags": []any{"Founder", "Manufacturing"}}})["record"].(map[string]any)
	mustCall(t, a, "upsert_record", map[string]any{"object": "people", "values": map[string]any{"name": "Bob Stone", "tags": "Founder"}})
	conditions := []any{map[string]any{"attribute": "tags", "operator": "is", "value": "Founder"}, map[string]any{"attribute": "tags", "operator": "is", "value": "Manufacturing"}}
	args := map[string]any{"object": "people", "name": " Manufacturing founders ", "query": " Stone ", "filters": conditions}
	saved := mustCall(t, a, "save_filter", args)
	if saved["name"] != "Manufacturing founders" || saved["query"] != "Stone" || !reflect.DeepEqual(saved["filters"], conditions) {
		t.Fatalf("saved filter: %v", saved)
	}
	reconnected := e.session(t, key)
	listed := mustCall(t, reconnected, "list_saved_filters", map[string]any{"object": "people"})["filters"].([]any)
	if len(listed) != 1 || !reflect.DeepEqual(listed[0], saved) {
		t.Fatalf("filter did not survive reconnection: %v", listed)
	}
	for _, scope := range []struct {
		session *mcp.ClientSession
		object  string
	}{{b, "people"}, {a, "companies"}} {
		if got := mustCall(t, scope.session, "list_saved_filters", map[string]any{"object": scope.object})["filters"].([]any); len(got) != 0 {
			t.Fatalf("saved filter crossed workspace/object boundary: %v", got)
		}
		if _, failure := call(t, scope.session, "save_filter", map[string]any{"object": scope.object, "id": saved["id"], "name": "Stolen"}); failure == "" {
			t.Fatal("saved filter was moved across a boundary")
		}
	}
	search := map[string]any{"object": "people", "query": saved["query"], "filters": saved["filters"]}
	result := mustCall(t, reconnected, "search_records", search)
	found := result["records"].([]any)
	if len(found) != 1 || found[0].(map[string]any)["id"] != person["id"] {
		t.Fatalf("applying the saved filter: %v", found)
	}
	uri, err := url.Parse(result["resource_uri"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip []any
	if err := json.Unmarshal([]byte(uri.Query().Get("filters")), &roundTrip); err != nil || !reflect.DeepEqual(roundTrip, conditions) {
		t.Fatalf("resource lost conditions: %s %v", uri, err)
	}
	resource, err := a.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri.String()})
	if err != nil || len(resource.Contents) != 1 || resource.Contents[0].MIMEType != "text/html;profile=mcp-app" {
		t.Fatalf("filtered resource: %v %v", resource, err)
	}
	args["name"] = "manufacturing FOUNDERS"
	if _, failure := call(t, a, "save_filter", args); failure == "" {
		t.Fatal("duplicate name accepted")
	}
	args["id"] = saved["id"]
	args["name"] = "Founders"
	args["filters"] = conditions[:1]
	updated := mustCall(t, a, "save_filter", args)
	if updated["id"] != saved["id"] || updated["name"] != "Founders" || len(updated["filters"].([]any)) != 1 {
		t.Fatalf("updating a saved filter: %v", updated)
	}
	if _, failure := call(t, b, "delete_saved_filter", map[string]any{"id": saved["id"]}); failure == "" {
		t.Fatal("another workspace deleted the filter")
	}
	mustCall(t, a, "delete_saved_filter", map[string]any{"id": saved["id"]})
	if got := mustCall(t, a, "list_saved_filters", map[string]any{"object": "people"})["filters"].([]any); len(got) != 0 {
		t.Fatalf("deleted filter still listed: %v", got)
	}
	if _, failure := call(t, a, "save_filter", args); failure == "" {
		t.Fatal("saving a deleted filter recreated it")
	}
	if got := mustCall(t, a, "search_records", map[string]any{"object": "people"})["records"].([]any); len(got) != 2 {
		t.Fatalf("deleting a filter affected its people: %v", got)
	}
}

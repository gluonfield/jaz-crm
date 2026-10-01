package mcpapi_test

import (
	"context"
	"encoding/json"
	"net/url"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
